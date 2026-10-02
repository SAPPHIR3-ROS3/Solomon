package server

import (
	"io/fs"
	"net/http"
	"strings"

	guibundle "github.com/SAPPHIR3-ROS3/Solomon/v2026/gui"
)

// productionFrontend serves built assets and lets React handle client-side routes.
func productionFrontend() http.Handler {
	assets := guibundle.Assets()
	files := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(assets, path); err != nil {
			// Missing static files must not receive HTML (especially old JS bundles).
			if strings.Contains(path, ".") || strings.HasPrefix(path, "assets/") {
				http.NotFound(w, r)
				return
			}
			copy := r.Clone(r.Context())
			copy.URL.Path = "/"
			files.ServeHTTP(w, copy)
			return
		}
		files.ServeHTTP(w, r)
	})
}
