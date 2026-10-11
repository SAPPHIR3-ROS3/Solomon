package server

import (
	"net/http"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

func handleNotifications(w http.ResponseWriter, r *http.Request) {
	entries, err := logging.ReadNotifications()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, entries)
}
