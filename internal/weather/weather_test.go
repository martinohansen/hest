package weather

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetch(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantErr    bool
		wantContains string
	}{
		{
			name: "sunny weather",
			response: `{
				"daily": {
					"weather_code": [0],
					"temperature_2m_max": [22.5]
				}
			}`,
			statusCode:   200,
			wantErr:      false,
			wantContains: "☀️ 22°",
		},
		{
			name: "rainy weather",
			response: `{
				"daily": {
					"weather_code": [61],
					"temperature_2m_max": [10.3]
				}
			}`,
			statusCode:   200,
			wantErr:      false,
			wantContains: "🌧️ 10°",
		},
		{
			name:       "api error",
			response:   `{}`,
			statusCode: 500,
			wantErr:    true,
		},
		{
			name: "empty data",
			response: `{
				"daily": {
					"weather_code": [],
					"temperature_2m_max": []
				}
			}`,
			statusCode: 200,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			// Override the fetch to use our test server
			result, err := fetchFromURL(server.URL, server.Client(), time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Fetch() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && result != tt.wantContains {
				t.Errorf("Fetch() = %q, want %q", result, tt.wantContains)
			}
		})
	}
}
