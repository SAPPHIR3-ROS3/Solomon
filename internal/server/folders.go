package server

import (
	"errors"
	"net/http"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

func (a *projectAPI) handleFolderState(w http.ResponseWriter, r *http.Request, projectID string) {
	panel := r.URL.Query().Get("panel")
	switch panel {
	case "chats", "chat-files", "editor":
	default:
		writeAPIError(w, http.StatusBadRequest, errors.New("invalid folder panel"))
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
		state, err := database.ReadFolderState(r.Context(), home, projectID, panel)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, state)
	case http.MethodPut:
		var state *database.FolderState
		if err := decodeJSONBody(w, r, &state, 1<<20); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		if state == nil {
			writeAPIError(w, http.StatusBadRequest, errors.New("missing folder state"))
			return
		}
		if err := database.WriteFolderState(r.Context(), home, projectID, panel, *state); err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	}
}
