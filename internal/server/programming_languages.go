package server

import (
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/programming"
)

type programmingLanguageRow struct {
	ID string `json:"id"`
	config.ProgrammingLanguage
	Server  programming.Server   `json:"server"`
	Servers []programming.Server `json:"servers"`
}

func handleProgrammingLanguages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodPost || r.Method == http.MethodPatch {
		var request struct {
			ID string `json:"id"`
			config.ProgrammingLanguage
		}
		if err := decodeJSONBody(w, r, &request, 65536); err != nil {
			writeAPIError(w, 400, err)
			return
		}
		if _, err := config.UpdateProgrammingLanguage(request.ID, request.ProgrammingLanguage, r.Method == http.MethodPost); err != nil {
			writeAPIError(w, 400, err)
			return
		}
	} else if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, POST, PATCH")
		writeAPIError(w, 405, errors.New("method not allowed"))
		return
	}
	languages, err := config.ReadProgrammingLanguages()
	if err != nil {
		writeAPIError(w, 500, err)
		return
	}
	rows := make([]programmingLanguageRow, 0, len(languages))
	for id, language := range languages {
		rows = append(rows, programmingLanguageRow{id, language, programming.Resolve(id, language), programming.InstalledServers(id, language)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	writeJSON(w, 200, rows)
}

func handleLanguageDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, 405, errors.New("method not allowed"))
		return
	}
	root, err := registeredProjectRoot(r.URL.Query().Get("project"))
	if err != nil {
		writeAPIError(w, 404, err)
		return
	}
	target, err := safeWorkspacePath(root, r.URL.Query().Get("path"))
	if err != nil {
		writeAPIError(w, 400, err)
		return
	}
	uri := func(path string) string {
		path = filepath.ToSlash(path)
		if strings.HasPrefix(path, "//") {
			host, tail, _ := strings.Cut(strings.TrimPrefix(path, "//"), "/")
			return (&url.URL{Scheme: "file", Host: host, Path: "/" + tail}).String()
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		return (&url.URL{Scheme: "file", Path: path}).String()
	}
	writeJSON(w, 200, map[string]string{"rootUri": uri(root), "uri": uri(target)})
}
