package server

import (
	"errors"
	"net/http"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

func (a *projectAPI) handlePDFPreferences(w http.ResponseWriter, r *http.Request, projectID string) {
	home, err := paths.SolomonHome()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		prefs, err := database.ReadPDFPreferences(r.Context(), home, projectID)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, prefs)
	case http.MethodPut:
		var prefs database.PDFPreferences
		if err := decodeJSONBody(w, r, &prefs, 4096); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		if (prefs.Direction != "vertical" && prefs.Direction != "horizontal") ||
			prefs.Zoom < 0.25 || prefs.Zoom > 3 ||
			(prefs.FitMode != nil && *prefs.FitMode != "page" && *prefs.FitMode != "width") {
			writeAPIError(w, http.StatusBadRequest, errors.New("invalid PDF preferences"))
			return
		}
		if err := database.WritePDFPreferences(r.Context(), home, projectID, prefs); err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	}
}
