package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeHandler records that it was called.
type fakeHandler struct{ called bool }

func (f *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.called = true
	w.WriteHeader(http.StatusOK)
}

func TestHTTPSecurityNoTokenAllowsAnonymous(t *testing.T) {
	fh := &fakeHandler{}
	h := HTTPSecurity{}.Wrap(fh)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/graph", nil)
	h.ServeHTTP(rec, req)

	if !fh.called {
		t.Fatalf("inner handler should have been called when no token configured")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("default Access-Control-Allow-Origin = %q, want *", got)
	}
}

func TestHTTPSecurityTokenRequiredRejectsMissingHeader(t *testing.T) {
	fh := &fakeHandler{}
	h := HTTPSecurity{Token: "s3cret"}.Wrap(fh)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/graph", nil)
	h.ServeHTTP(rec, req)

	if fh.called {
		t.Errorf("inner handler should not have been called without auth header")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestHTTPSecurityTokenRequiredRejectsWrongToken(t *testing.T) {
	fh := &fakeHandler{}
	h := HTTPSecurity{Token: "s3cret"}.Wrap(fh)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/graph", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	h.ServeHTTP(rec, req)

	if fh.called {
		t.Errorf("inner handler should not have been called with wrong token")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestHTTPSecurityTokenRequiredAcceptsValidToken(t *testing.T) {
	fh := &fakeHandler{}
	h := HTTPSecurity{Token: "s3cret"}.Wrap(fh)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/graph", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	h.ServeHTTP(rec, req)

	if !fh.called {
		t.Errorf("inner handler should have been called with valid token")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// EventSource in the browser can't send custom headers, so the middleware
// also accepts the token via a ?token= query param. The behavior is
// symmetric: same valid token → 200, anything else → 401.
func TestHTTPSecurityTokenRequiredAcceptsQueryParam(t *testing.T) {
	fh := &fakeHandler{}
	h := HTTPSecurity{Token: "s3cret"}.Wrap(fh)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/events/graph?token=s3cret", nil)
	h.ServeHTTP(rec, req)

	if !fh.called {
		t.Errorf("inner handler should have been called with valid token query param")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestHTTPSecurityTokenRequiredRejectsWrongQueryParam(t *testing.T) {
	fh := &fakeHandler{}
	h := HTTPSecurity{Token: "s3cret"}.Wrap(fh)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/events/graph?token=wrong", nil)
	h.ServeHTTP(rec, req)

	if fh.called {
		t.Errorf("inner handler should not have been called with wrong token query param")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// CORS preflight should be honored without requiring auth, otherwise browsers
// can't even discover that the endpoint exists.
func TestHTTPSecurityPreflightSkipsAuth(t *testing.T) {
	fh := &fakeHandler{}
	h := HTTPSecurity{AllowOrigin: "https://dashboard.example", Token: "s3cret"}.Wrap(fh)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/v1/graph", nil)
	h.ServeHTTP(rec, req)

	if fh.called {
		t.Errorf("inner handler should not have been called on preflight")
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://dashboard.example" {
		t.Errorf("Access-Control-Allow-Origin = %q, want https://dashboard.example", got)
	}
	if rec.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Errorf("preflight should advertise allowed methods")
	}
}

// When the security wrapper is configured with protected prefixes, paths
// outside the prefix list serve without auth even when a token is set. This
// is what makes the static UI loadable so the JS can later prompt for a
// token to talk to /v1/*.
func TestHTTPSecurityProtectedPrefixOnlyGatesMatchingPaths(t *testing.T) {
	fh := &fakeHandler{}
	h := HTTPSecurity{Token: "s3cret", ProtectedPrefixes: []string{"/v1/"}}.Wrap(fh)

	cases := []struct {
		path       string
		wantStatus int
		wantCalled bool
	}{
		{"/v1/graph", http.StatusUnauthorized, false},
		{"/", http.StatusOK, true},
		{"/index.html", http.StatusOK, true},
		{"/style.css", http.StatusOK, true},
	}
	for _, tc := range cases {
		fh.called = false
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", tc.path, nil)
		h.ServeHTTP(rec, req)
		if rec.Code != tc.wantStatus {
			t.Errorf("%s: status = %d, want %d", tc.path, rec.Code, tc.wantStatus)
		}
		if fh.called != tc.wantCalled {
			t.Errorf("%s: inner called = %v, want %v", tc.path, fh.called, tc.wantCalled)
		}
	}
}

func TestHTTPSecurityCustomOriginPropagates(t *testing.T) {
	fh := &fakeHandler{}
	h := HTTPSecurity{AllowOrigin: "https://dashboard.example"}.Wrap(fh)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/graph", nil)
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://dashboard.example" {
		t.Errorf("Access-Control-Allow-Origin = %q, want https://dashboard.example", got)
	}
}
