package server

import (
	"errors"
	"net/http"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func handleGUISettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		gui, err := config.ReadGUISettings()
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, gui)
	case http.MethodPatch:
		var request struct {
			Chat config.GUIChatSettings `json:"chat"`
		}
		if err := decodeJSONBody(w, r, &request, 4096); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		if request.Chat.AutoCloseToolCalls == nil && request.Chat.StartToolCallsCollapsed == nil {
			writeAPIError(w, http.StatusBadRequest, errors.New("at least one chat preference is required"))
			return
		}
		gui, err := config.UpdateGUIChatSettings(request.Chat)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, gui)
	default:
		w.Header().Set("Allow", "GET, PATCH")
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	}
}
