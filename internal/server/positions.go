package server

import (
	"errors"
	"io/fs"
	"net/http"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

func (a *projectAPI) handleFilePosition(w http.ResponseWriter, r *http.Request, projectID string) {
	path, view := r.URL.Query().Get("path"), r.URL.Query().Get("view")
	if !fs.ValidPath(path) || path == "." || (view != "source" && view != "preview") {
		writeAPIError(w, http.StatusBadRequest, errors.New("invalid file position key"))
		return
	}
	home, err := paths.SolomonHome()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		position, err := database.ReadFilePosition(r.Context(), home, projectID, path, view)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, position)
	case http.MethodPut:
		var position *database.FilePosition
		if err := decodeJSONBody(w, r, &position, 4096); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		if position == nil || position.Top < 0 || position.Left < 0 || position.Anchor < 0 || position.Head < 0 ||
			position.Page < 0 || (position.ScrollAnchor != nil && *position.ScrollAnchor < 0) {
			writeAPIError(w, http.StatusBadRequest, errors.New("invalid file position"))
			return
		}
		if err := database.WriteFilePosition(r.Context(), home, projectID, path, view, *position); err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	}
}
