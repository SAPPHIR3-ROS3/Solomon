package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func staticHandler(dir string, api http.Handler) http.Handler {
	root := filepath.Clean(dir)
	fs := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/") || r.URL.Path == "/health" {
			api.ServeHTTP(w, r)
			return
		}
		p := filepath.Join(root, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		index := filepath.Join(root, "index.html")
		if _, err := os.Stat(index); err == nil {
			http.ServeFile(w, r, index)
			return
		}
		api.ServeHTTP(w, r)
	})
}
