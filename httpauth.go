package main

import (
	"crypto/subtle"
	"net/http"
)

// HTTPSecurity holds the cross-cutting HTTP knobs (CORS, optional bearer
// auth) shared by the mapper and aggregator handlers. Construct once in
// main and pass into both HTTPApi.Start calls.
type HTTPSecurity struct {
	// AllowOrigin is the value sent in Access-Control-Allow-Origin. Empty or
	// "*" preserves the historical wide-open default so existing dashboards
	// keep working; set to a specific origin to lock it down.
	AllowOrigin string
	// Token, if non-empty, makes the middleware require an
	// "Authorization: Bearer <Token>" header on every wrapped request.
	Token string
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
		if s.Token != "" {
			const prefix = "Bearer "
			got := r.Header.Get("Authorization")
			// Constant-time compare to avoid timing-based token oracle.
			if len(got) < len(prefix) || got[:len(prefix)] != prefix ||
				subtle.ConstantTimeCompare([]byte(got[len(prefix):]), []byte(s.Token)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}
