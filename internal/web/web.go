// Package web serves the built React client out of the binary.
package web

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var assets embed.FS

// Handler serves the client. Hashed asset files are cached for a year; the
// entry point never is, so a new build is picked up on the next load.
//
// Unknown paths fall through to index.html because the client routes on the
// history API — a refresh on /settings must not 404.
func Handler() http.Handler {
	dist, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err)
	}
	index, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		// A binary built without `task web` has no UI. The API still works, so
		// say so plainly instead of failing to start.
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "web client not built: run `task web` before building", http.StatusNotFound)
		})
	}

	files := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			serveIndex(w, index)
			return
		}
		if _, err := fs.Stat(dist, path); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				serveIndex(w, index)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if strings.HasPrefix(path, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}

func serveIndex(w http.ResponseWriter, index []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(index)
}
