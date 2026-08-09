package main

import (
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	perIPFailureLimit  = 5
	globalFailureLimit = 100
	failureWindow      = 10 * time.Minute
	blockDuration      = 15 * time.Minute
)

type authEntry struct {
	failures     []time.Time
	blockedUntil time.Time
}

type authLimiter struct {
	mu                 sync.Mutex
	entries            map[string]*authEntry
	globalFailures     []time.Time
	globalBlockedUntil time.Time
	now                func() time.Time
}

func newAuthLimiter(now func() time.Time) *authLimiter {
	return &authLimiter{
		entries: make(map[string]*authEntry),
		now:     now,
	}
}

// evaluate atomically checks active blocks and records a failed attempt. The
// attempt that reaches a limit is allowed to receive its normal 401 response;
// the block applies to later attempts.
func (l *authLimiter) evaluate(ip string, failed bool) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.prune(now)

	blockedUntil := l.globalBlockedUntil
	if entry := l.entries[ip]; entry != nil && entry.blockedUntil.After(blockedUntil) {
		blockedUntil = entry.blockedUntil
	}
	if blockedUntil.After(now) {
		return blockedUntil.Sub(now), true
	}
	if !failed {
		return 0, false
	}

	entry := l.entries[ip]
	if entry == nil {
		entry = &authEntry{}
		l.entries[ip] = entry
	}
	entry.failures = append(entry.failures, now)
	l.globalFailures = append(l.globalFailures, now)

	if len(entry.failures) >= perIPFailureLimit {
		entry.blockedUntil = now.Add(blockDuration)
	}
	if len(l.globalFailures) >= globalFailureLimit {
		l.globalBlockedUntil = now.Add(blockDuration)
	}

	return 0, false
}

func (l *authLimiter) prune(now time.Time) {
	cutoff := now.Add(-failureWindow)
	l.globalFailures = pruneFailures(l.globalFailures, cutoff)
	if !l.globalBlockedUntil.After(now) {
		l.globalBlockedUntil = time.Time{}
	}

	for ip, entry := range l.entries {
		entry.failures = pruneFailures(entry.failures, cutoff)
		if !entry.blockedUntil.After(now) {
			entry.blockedUntil = time.Time{}
		}
		if len(entry.failures) == 0 && entry.blockedUntil.IsZero() {
			delete(l.entries, ip)
		}
	}
}

func pruneFailures(failures []time.Time, cutoff time.Time) []time.Time {
	firstCurrent := 0
	for firstCurrent < len(failures) && !failures[firstCurrent].After(cutoff) {
		firstCurrent++
	}
	return failures[firstCurrent:]
}

func credentials(r *http.Request) (string, bool) {
	user, pass, ok := r.BasicAuth()
	if !ok || pass != password() || user == "" {
		return "", false
	}
	return strings.TrimSpace(user), true
}

// requireAuth identifies and authorizes user from basic auth credentials
func requireAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	user, ok := credentials(r)
	if !ok {
		unauthorized(w)
		return "", false
	}
	return user, true
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Hest"`)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte("authorization required"))
}

func password() string {
	if p := strings.TrimSpace(os.Getenv("HEST_PASSWORD")); p != "" {
		return p
	}
	return "hest"
}

// ensureAuthAndForm ensures that the request is an authorized POST with valid
// form data. Failed authentication attempts are subject to rate limiting.
func (a *App) ensureAuthAndForm(w http.ResponseWriter, r *http.Request) (username string, ok bool) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return "", false
	}

	username, valid := credentials(r)
	retryAfter, blocked := a.authLimiter.evaluate(clientIP(r), !valid)
	if blocked {
		writeTooManyRequests(w, retryAfter)
		return "", false
	}
	if !valid {
		unauthorized(w)
		return "", false
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return "", false
	}
	return username, true
}

func clientIP(r *http.Request) string {
	first, _, _ := strings.Cut(r.Header.Get("X-Forwarded-For"), ",")
	if forwarded, err := netip.ParseAddr(strings.TrimSpace(first)); err == nil {
		return forwarded.Unmap().String()
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	if peer, err := netip.ParseAddr(host); err == nil {
		return peer.Unmap().String()
	}
	return host
}

func writeTooManyRequests(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := max(int64(1), int64((retryAfter+time.Second-1)/time.Second))
	w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
	http.Error(w, "too many authentication attempts", http.StatusTooManyRequests)
}
