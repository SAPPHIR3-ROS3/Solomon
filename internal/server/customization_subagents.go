package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/roles"
)

func (a *customizationAPI) handleSubagents(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		a.handleCreateSubagent(w, r)
		return
	}
	if r.Method == http.MethodGet {
		items, err := loadSubagents()
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"subagents": items})
		return
	}
	writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
}

func (a *customizationAPI) handleCreateSubagent(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Provider string                 `json:"provider"`
		Model    string                 `json:"model"`
		Detail   string                 `json:"detail"`
		Scores   []apiScorePatchRequest `json:"scores"`
	}
	if err := decodeJSONBody(w, r, &request, 256<<10); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	cfg, err := config.Load()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	entry := config.SubagentRoleConfig{
		Provider:    strings.TrimSpace(request.Provider),
		Model:       strings.TrimSpace(request.Model),
		Description: strings.TrimSpace(request.Detail),
		Scores:      map[string]int{},
	}
	for _, score := range request.Scores {
		id := strings.TrimSpace(score.ID)
		if _, exists := entry.Scores[id]; exists {
			writeAPIError(w, http.StatusBadRequest, fmt.Errorf("duplicate characteristic %q", id))
			return
		}
		entry.Scores[id] = score.Value
	}
	for _, id := range cfg.Roles.Table.Characteristics {
		if _, exists := entry.Scores[id]; !exists {
			writeAPIError(w, http.StatusBadRequest, fmt.Errorf("score required for %q", id))
			return
		}
	}
	cfg.Roles.Subagent = append(cfg.Roles.Subagent, entry)
	if err := config.ValidateRoles(r.Context(), cfg); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	if err := config.Save(cfg); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	items, err := loadSubagents()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subagents": items})
}

func (a *customizationAPI) handleUpdateSubagent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		Detail string                 `json:"detail"`
		ID     string                 `json:"id"`
		Scores []apiScorePatchRequest `json:"scores"`
	}
	if err := decodeJSONBody(w, r, &request, 256<<10); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(request.ID) == "" {
		writeAPIError(w, http.StatusBadRequest, errors.New("id is required"))
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	index, err := subagentIndex(request.ID)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	cfg, err := config.Load()
	if err != nil || index >= len(cfg.Roles.Subagent) {
		if err == nil {
			err = errors.New("subagent not found")
		}
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	cfg.Roles.Subagent[index].Description = strings.TrimSpace(request.Detail)
	if cfg.Roles.Subagent[index].Scores == nil {
		cfg.Roles.Subagent[index].Scores = map[string]int{}
	}
	for _, score := range request.Scores {
		id := strings.TrimSpace(score.ID)
		if !roles.IsKnownCharacteristic(id) {
			writeAPIError(w, http.StatusBadRequest, fmt.Errorf("unknown characteristic %q", id))
			return
		}
		if err := roles.ValidateScoreValue(id, score.Value); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		cfg.Roles.Subagent[index].Scores[id] = score.Value
	}
	if err := config.Save(cfg); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	items, err := loadSubagents()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subagents": items})
}

type apiScorePatchRequest struct {
	ID    string `json:"id"`
	Value int    `json:"value"`
}

func (a *customizationAPI) handleDeleteSubagent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		ID string `json:"id"`
	}
	if err := decodeJSONBody(w, r, &request, 4096); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	index, err := subagentIndex(request.ID)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	cfg, err := config.Load()
	if err != nil || index >= len(cfg.Roles.Subagent) {
		if err == nil {
			err = errors.New("subagent not found")
		}
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	cfg.Roles.Subagent = append(cfg.Roles.Subagent[:index], cfg.Roles.Subagent[index+1:]...)
	if err := config.Save(cfg); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	items, err := loadSubagents()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subagents": items})
}

func loadSubagents() ([]apiCustomizationItem, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	items := make([]apiCustomizationItem, 0, len(cfg.Roles.Subagent))
	for index, role := range cfg.Roles.Subagent {
		provider := strings.TrimSpace(role.Provider)
		model := strings.TrimSpace(role.Model)
		title := model
		if title == "" {
			title = fmt.Sprintf("subagent-%d", index+1)
		}
		scores := make([]apiCustomizationScore, 0, len(cfg.Roles.Table.Characteristics))
		for _, characteristic := range cfg.Roles.Table.Characteristics {
			scores = append(scores, apiCustomizationScore{ID: characteristic, Label: roles.CharacteristicLabel(characteristic), Value: role.Scores[characteristic]})
		}
		items = append(items, apiCustomizationItem{
			Badge:  provider,
			Detail: strings.TrimSpace(role.Description),
			ID:     fmt.Sprintf("%s:%s:%d", provider, model, index),
			Scores: scores,
			Title:  title,
		})
	}
	return items, nil
}

func subagentIndex(id string) (int, error) {
	index := strings.LastIndex(strings.TrimSpace(id), ":")
	if index < 0 || index == len(strings.TrimSpace(id))-1 {
		return -1, errors.New("invalid subagent id")
	}
	value, err := strconv.Atoi(strings.TrimSpace(id)[index+1:])
	if err != nil || value < 0 {
		return -1, errors.New("invalid subagent id")
	}
	return value, nil
}
