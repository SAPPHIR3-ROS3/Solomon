package server

import (
	"encoding/json"
	"errors"
	displaysyntax "github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/syntax"
	"io"
	"net/http"
)

const maxGoFormatBody = 1 << 20

func handleFormatGo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	defer r.Body.Close()
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGoFormatBody))
	if err != nil {
		status := http.StatusBadRequest
		var limitError *http.MaxBytesError
		if errors.As(err, &limitError) {
			status = http.StatusRequestEntityTooLarge
		}
		writeAPIError(w, status, err)
		return
	}
	var request struct {
		Source *string `json:"source"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, errors.New("invalid JSON request"))
		return
	}
	if request.Source == nil {
		writeAPIError(w, http.StatusBadRequest, errors.New("source must be a string"))
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Source string `json:"source"`
	}{Source: formatGoDisplay(*request.Source)})
}

func formatGoDisplay(source string) string { return displaysyntax.FormatGo(source) }
