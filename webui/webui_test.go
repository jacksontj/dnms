package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandlerServesIndexAndAssets pins the embed.FS contract: the handler
// must serve index.html at /, and the JS/CSS/vendor assets by their on-disk
// names with the right content types. A missing-asset bug at build time
// would otherwise only show up as a blank page in the browser.
func TestHandlerServesIndexAndAssets(t *testing.T) {
	h := Handler()
	cases := []struct {
		path           string
		wantStatus     int
		wantContentSub string // substring of Content-Type
		wantBodySub    string // substring expected in the body
	}{
		{"/", 200, "text/html", "<title>dnms</title>"},
		// http.FileServer rewrites /index.html → / as a canonical-URL
		// redirect; lock that in so a future refactor doesn't accidentally
		// expose both paths.
		{"/index.html", 301, "", ""},
		{"/style.css", 200, "text/css", "--bg:"},
		{"/app.js", 200, "javascript", "cytoscape"},
		{"/vendor/cytoscape.min.js", 200, "javascript", "Cytoscape"},
		{"/nope.txt", 404, "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("%s: status = %d, want %d", tc.path, rec.Code, tc.wantStatus)
			}
			if tc.wantStatus != http.StatusOK {
				return
			}
			if got := rec.Header().Get("Content-Type"); !strings.Contains(got, tc.wantContentSub) {
				t.Errorf("%s: Content-Type = %q, want substring %q", tc.path, got, tc.wantContentSub)
			}
			if !strings.Contains(rec.Body.String(), tc.wantBodySub) {
				t.Errorf("%s: body missing substring %q", tc.path, tc.wantBodySub)
			}
		})
	}
}
