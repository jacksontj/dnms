// Package webui packages the dnms HTML/JS dashboard into the binary via
// embed.FS so a single `dnms` invocation serves both the JSON API and the
// browser UI without any external static-file setup.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

// dist is the bundled dashboard. Keep the assets tree small — anything we
// bundle here goes into the binary.
//
//go:embed index.html style.css app.js vendor/cytoscape.min.js
var dist embed.FS

// Handler returns an http.Handler that serves the embedded dashboard from
// the package root. The caller decides the mount path (typically "/").
func Handler() http.Handler {
	// Strip the "" prefix from the embed sub-FS so requests for "/style.css"
	// resolve to "style.css" inside the embed.
	sub, err := fs.Sub(dist, ".")
	if err != nil {
		// Embed sub-FS errors are programmer errors, not runtime errors;
		// panic so a misconfigured build fails loudly at startup rather
		// than silently 404'ing every dashboard request.
		panic("webui: bad embed FS: " + err.Error())
	}
	return http.FileServer(http.FS(sub))
}
