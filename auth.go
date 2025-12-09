package main

import (
	"net/http"
	"os"
	"strings"
)

// requireAuth identifies and authorizes user from basic auth credentials
func requireAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	user, pass, ok := r.BasicAuth()
	if !ok || pass != password() || user == "" {
		unauthorized(w)
		return "", false
	}
	return strings.TrimSpace(user), true
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

// ensureAuthAndForm ensures that the request is a POST with valid form data and
// authorized user. Returns the identify of the user and if they are authorized.
func ensureAuthAndForm(w http.ResponseWriter, r *http.Request) (username string, ok bool) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return "", false
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return "", false
	}
	return requireAuth(w, r)
}
