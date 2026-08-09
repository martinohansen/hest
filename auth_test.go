package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/martinohansen/hest/internal/db"
)

type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
}

func TestAuthLimiterPerIPBlock(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)}
	limiter := newAuthLimiter(clock.Now)

	for attempt := 1; attempt <= perIPFailureLimit; attempt++ {
		if retryAfter, blocked := limiter.evaluate("192.0.2.1", true); blocked {
			t.Fatalf("attempt %d blocked with retry-after %s", attempt, retryAfter)
		}
	}

	retryAfter, blocked := limiter.evaluate("192.0.2.1", false)
	if !blocked {
		t.Fatal("valid sixth attempt was not blocked")
	}
	if retryAfter != blockDuration {
		t.Fatalf("retry-after = %s, want %s", retryAfter, blockDuration)
	}

	clock.Advance(5 * time.Minute)
	retryAfter, blocked = limiter.evaluate("192.0.2.1", true)
	if !blocked || retryAfter != 10*time.Minute {
		t.Fatalf("after 5 minutes: blocked = %v, retry-after = %s", blocked, retryAfter)
	}

	clock.Advance(10 * time.Minute)
	if retryAfter, blocked = limiter.evaluate("192.0.2.1", false); blocked {
		t.Fatalf("attempt remained blocked with retry-after %s", retryAfter)
	}
	if len(limiter.entries) != 0 {
		t.Fatalf("entries after expiry = %d, want 0", len(limiter.entries))
	}
}

func TestAuthLimiterSuccessDoesNotResetFailures(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)}
	limiter := newAuthLimiter(clock.Now)

	for range perIPFailureLimit - 1 {
		limiter.evaluate("192.0.2.2", true)
	}
	if _, blocked := limiter.evaluate("192.0.2.2", false); blocked {
		t.Fatal("successful attempt was blocked before the limit")
	}
	if _, blocked := limiter.evaluate("192.0.2.2", true); blocked {
		t.Fatal("threshold failure should receive 401")
	}
	if _, blocked := limiter.evaluate("192.0.2.2", false); !blocked {
		t.Fatal("success reset recent failures")
	}
}

func TestAuthLimiterRollingWindow(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)}
	limiter := newAuthLimiter(clock.Now)

	for range perIPFailureLimit - 1 {
		limiter.evaluate("192.0.2.3", true)
	}
	clock.Advance(failureWindow)
	if _, blocked := limiter.evaluate("192.0.2.3", true); blocked {
		t.Fatal("expired failures caused a block")
	}

	entry := limiter.entries["192.0.2.3"]
	if entry == nil || len(entry.failures) != 1 {
		t.Fatalf("current failures = %v, want one", entry)
	}
}

func TestAuthLimiterGlobalBlock(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)}
	limiter := newAuthLimiter(clock.Now)

	for attempt := range globalFailureLimit {
		ip := "client-" + strconv.Itoa(attempt)
		if retryAfter, blocked := limiter.evaluate(ip, true); blocked {
			t.Fatalf("attempt %d blocked with retry-after %s", attempt+1, retryAfter)
		}
	}

	retryAfter, blocked := limiter.evaluate("successful-client", false)
	if !blocked || retryAfter != blockDuration {
		t.Fatalf("global block = %v, retry-after = %s", blocked, retryAfter)
	}
	if len(limiter.globalFailures) != globalFailureLimit {
		t.Fatalf("blocked attempt changed failure count to %d", len(limiter.globalFailures))
	}

	clock.Advance(blockDuration)
	if retryAfter, blocked = limiter.evaluate("successful-client", false); blocked {
		t.Fatalf("global block remained active with retry-after %s", retryAfter)
	}
	if len(limiter.globalFailures) != 0 || len(limiter.entries) != 0 {
		t.Fatalf("expired state remains: global = %d, entries = %d",
			len(limiter.globalFailures), len(limiter.entries))
	}
}

func TestAuthLimiterUsesLaterBlockExpiry(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)}
	limiter := newAuthLimiter(clock.Now)

	for range perIPFailureLimit {
		limiter.evaluate("192.0.2.5", true)
	}
	clock.Advance(5 * time.Minute)
	for attempt := perIPFailureLimit; attempt < globalFailureLimit; attempt++ {
		limiter.evaluate("client-"+strconv.Itoa(attempt), true)
	}

	retryAfter, blocked := limiter.evaluate("192.0.2.5", false)
	if !blocked || retryAfter != blockDuration {
		t.Fatalf("combined block = %v, retry-after = %s, want %s",
			blocked, retryAfter, blockDuration)
	}
}

func TestAuthLimiterExpiresInactiveEntries(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)}
	limiter := newAuthLimiter(clock.Now)
	limiter.evaluate("192.0.2.4", true)

	clock.Advance(failureWindow)
	limiter.evaluate("successful-client", false)

	if len(limiter.entries) != 0 {
		t.Fatalf("entries = %d, want 0", len(limiter.entries))
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		forwarded  string
		want       string
	}{
		{name: "forwarded IPv4", remoteAddr: "127.0.0.1:1234", forwarded: "192.0.2.10", want: "192.0.2.10"},
		{name: "first forwarded address", remoteAddr: "127.0.0.1:1234", forwarded: "2001:db8::1, 192.0.2.10", want: "2001:db8::1"},
		{name: "canonical mapped IPv4", remoteAddr: "127.0.0.1:1234", forwarded: "::ffff:192.0.2.10", want: "192.0.2.10"},
		{name: "invalid forwarded address", remoteAddr: "198.51.100.7:4321", forwarded: "unknown", want: "198.51.100.7"},
		{name: "missing forwarded address", remoteAddr: "[2001:db8::2]:4321", want: "2001:db8::2"},
		{name: "unstructured remote address", remoteAddr: "local-client", want: "local-client"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.RemoteAddr = tt.remoteAddr
			r.Header.Set("X-Forwarded-For", tt.forwarded)
			if got := clientIP(r); got != tt.want {
				t.Fatalf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteTooManyRequests(t *testing.T) {
	tests := []struct {
		name       string
		duration   time.Duration
		retryAfter string
	}{
		{name: "rounds up", duration: 1500 * time.Millisecond, retryAfter: "2"},
		{name: "minimum", duration: 0, retryAfter: "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeTooManyRequests(w, tt.duration)
			if w.Code != http.StatusTooManyRequests {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
			}
			if got := w.Header().Get("Retry-After"); got != tt.retryAfter {
				t.Fatalf("Retry-After = %q, want %q", got, tt.retryAfter)
			}
		})
	}
}

func TestListenAddress(t *testing.T) {
	tests := []struct {
		name string
		host string
		port string
		want string
	}{
		{name: "defaults", want: "localhost:8080"},
		{name: "host and port override", host: "0.0.0.0", port: "9090", want: "0.0.0.0:9090"},
		{name: "IPv6 override", host: "::1", want: "[::1]:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HEST_HOST", tt.host)
			t.Setenv("HEST_PORT", tt.port)
			if got := listenAddress(); got != tt.want {
				t.Fatalf("listenAddress() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProtectedWriteRoutesAreLimited(t *testing.T) {
	t.Setenv("HEST_PASSWORD", "correct-password")
	routes := []string{
		"/players/add",
		"/players/update",
		"/games/save",
		"/games/save-and-new",
		"/game/update",
		"/game/delete",
		"/seasons",
	}

	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			app := newApp(nil)
			for attempt := 1; attempt <= perIPFailureLimit+1; attempt++ {
				request := newFormRequest(route, "192.0.2.20")
				request.SetBasicAuth("player", "wrong-password")
				response := httptest.NewRecorder()
				app.routes().ServeHTTP(response, request)

				want := http.StatusUnauthorized
				if attempt > perIPFailureLimit {
					want = http.StatusTooManyRequests
				}
				if response.Code != want {
					t.Fatalf("attempt %d status = %d, want %d", attempt, response.Code, want)
				}
			}
		})
	}
}

func TestProtectedReadRoutesAreNotLimited(t *testing.T) {
	t.Setenv("HEST_PASSWORD", "correct-password")
	for _, route := range []string{"/game/edit", "/players", "/seasons"} {
		t.Run(route, func(t *testing.T) {
			app := newApp(nil)
			for attempt := 1; attempt <= perIPFailureLimit+1; attempt++ {
				request := httptest.NewRequest(http.MethodGet, route, nil)
				request.Header.Set("X-Forwarded-For", "192.0.2.21")
				request.SetBasicAuth("player", "wrong-password")
				response := httptest.NewRecorder()
				app.routes().ServeHTTP(response, request)
				if response.Code != http.StatusUnauthorized {
					t.Fatalf("attempt %d status = %d, want %d",
						attempt, response.Code, http.StatusUnauthorized)
				}
			}
		})
	}
}

func TestScoreRouteDoesNotContributeToLimit(t *testing.T) {
	t.Setenv("HEST_PASSWORD", "correct-password")
	store, err := db.Open(t.TempDir() + "/hest.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	app := newApp(store)

	for range perIPFailureLimit + 1 {
		request := newFormRequest("/new/score", "192.0.2.22")
		request.SetBasicAuth("player", "wrong-password")
		response := httptest.NewRecorder()
		app.routes().ServeHTTP(response, request)
		if response.Code == http.StatusUnauthorized || response.Code == http.StatusTooManyRequests {
			t.Fatalf("score route returned authentication status %d", response.Code)
		}
	}

	request := newFormRequest("/players/add", "192.0.2.22")
	request.SetBasicAuth("player", "wrong-password")
	response := httptest.NewRecorder()
	app.routes().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("first protected failure status = %d, want %d",
			response.Code, http.StatusUnauthorized)
	}
}

func newFormRequest(route, forwardedFor string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, route,
		strings.NewReader(url.Values{}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("X-Forwarded-For", forwardedFor)
	return request
}
