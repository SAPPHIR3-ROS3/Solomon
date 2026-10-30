package server

import (
	"errors"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"net/http"
)

func (a *modelAPI) handleDisconnectProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		Provider  string `json:"provider"`
		Confirmed bool   `json:"confirmed"`
	}
	if err := decodeJSONBody(w, r, &request, 32<<10); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	if !request.Confirmed {
		writeAPIError(w, http.StatusBadRequest, errors.New("confirmation required"))
		return
	}
	if _, err := config.RemoveProvider(request.Provider); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	a.invalidate()
	writeJSON(w, http.StatusOK, map[string]bool{"removed": true})
}
