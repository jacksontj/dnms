package main

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// HTTPSecurity holds the cross-cutting HTTP knobs (CORS, optional bearer
// auth) shared by the mapper and aggregator handlers. Construct once in
// main and pass into both HTTPApi.Start calls.
type HTTPSecurity struct {
	// AllowOrigin is the value sent in Access-Control-Allow-Origin. Empty or
	// "*" preserves the historical wide-open default so existing dashboards
	// keep working; set to a specific origin to lock it down.
	AllowOrigin string
	// Token, if non-empty, makes the middleware require either an
	// "Authorization: Bearer <Token>" header or a "?token=<Token>" query
	// parameter on protected requests.
	Token string
	// ProtectedPrefixes optionally restricts auth to paths starting with any
	// of these prefixes. Empty (the default) requires auth on every request
	// — backward-compatible for callers that wrap a JSON-only mux. Set to
	// e.g. []string{"/v1/"} when a single mux also serves a static UI: the
	// browser needs to load the JS to be able to prompt for the token in
	// the first place.
	ProtectedPrefixes []string
}

// Wrap installs CORS headers and (optionally) a bearer token check on top
// of the given handler. Returns h unchanged if no behavior is configured.
func (s HTTPSecurity) Wrap(h http.Handler) http.Handler {
	if s.AllowOrigin == "" {
		s.AllowOrigin = "*"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.AllowOrigin)
		// Preflight: short-circuit so OPTIONS doesn't get auth-rejected.
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if s.Token != "" && s.protected(r.URL.Path) && !s.authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (s HTTPSecurity) protected(path string) bool {
	if len(s.ProtectedPrefixes) == 0 {
		return true // wrap everything (backward-compat default)
	}
	for _, p := range s.ProtectedPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// authorized returns true if the request carries the configured token, either
// in the Authorization header (preferred) or in a ?token= query parameter.
// The query-param path exists for browser EventSource, which cannot send
// custom headers; same-origin SSE auth has no better cross-browser option.
func (s HTTPSecurity) authorized(r *http.Request) bool {
	if got := r.Header.Get("Authorization"); got != "" {
		const prefix = "Bearer "
		if len(got) >= len(prefix) && got[:len(prefix)] == prefix &&
			subtle.ConstantTimeCompare([]byte(got[len(prefix):]), []byte(s.Token)) == 1 {
			return true
		}
	}
	if got := r.URL.Query().Get("token"); got != "" &&
		subtle.ConstantTimeCompare([]byte(got), []byte(s.Token)) == 1 {
		return true
	}
	return false
}
