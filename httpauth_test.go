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
