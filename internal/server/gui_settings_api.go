package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
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
		var request config.GUISettingsPatch
		if err := decodeJSONBody(w, r, &request, 4096); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		if err := request.Validate(); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		gui, err := config.UpdateGUIPreferences(request)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, config.ErrInvalidServerDevelopment) {
				status = http.StatusBadRequest
			}
			writeAPIError(w, status, err)
			return
		}
		if raw, ok := request.Advanced["install_layout"]; ok {
			var layout string
			_ = json.Unmarshal(raw, &layout)
			go func() {
				if err := updater.SwitchToLayout(context.Background(), layout); err != nil {
					logging.Log(logging.WARNING_LOG_LEVEL, "install layout switch failed", logging.LogOptions{Params: map[string]any{"layout": layout, "err": err.Error()}})
				}
			}()
		}
		writeJSON(w, http.StatusOK, gui)
	default:
		w.Header().Set("Allow", "GET, PATCH")
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	}
}
