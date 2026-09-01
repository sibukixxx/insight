// Package web embeds the static frontend so the whole application ships as
// one binary. The frontend is plain HTML/CSS/JS (no build step) so it can
// be committed and embedded directly without a Node toolchain being part
// of the Go build.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// Handler serves the embedded frontend, falling back to index.html for any
// path that isn't a real file so client-side routing (hash-based) works on
// a hard refresh.
func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(sub, path); err != nil {
			r = r.Clone(r.Context())
			r.URL = &url.URL{Path: "/"}
		}
		fileServer.ServeHTTP(w, r)
	})
}
