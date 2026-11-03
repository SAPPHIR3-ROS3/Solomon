package server

import (
	"errors"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"net/http"
	"strings"
)

func (a *projectAPI) handleUserName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		UserName string `json:"userName"`
	}
	if err := decodeJSONBody(w, r, &request, 4096); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	request.UserName = strings.TrimSpace(request.UserName)
	if len(request.UserName) > 120 {
		writeAPIError(w, http.StatusBadRequest, errors.New("user name is too long"))
		return
	}
	cfg, err := config.Load()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	cfg.UserName = request.UserName
	if err := config.Save(cfg); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"userName": request.UserName})
}

func (a *projectAPI) handleReasoningEffort(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		ReasoningEffort string `json:"reasoningEffort"`
	}
	if err := decodeJSONBody(w, r, &request, 4096); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	effort, err := config.ParseReasoningEffortToken(request.ReasoningEffort)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := config.UpdateReasoningEffort(effort); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"reasoningEffort": effort})
}
