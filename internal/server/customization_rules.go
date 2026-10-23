package server

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/instructions"
)

type apiCustomizationRule struct {
	ID      int    `json:"id"`
	Text    string `json:"text"`
	Enabled bool   `json:"enabled"`
}

func (a *customizationAPI) handleRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		Text string `json:"text"`
	}
	if r.Method == http.MethodPost {
		if err := decodeJSONBody(w, r, &request, 64<<10); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		if strings.TrimSpace(request.Text) == "" {
			writeAPIError(w, http.StatusBadRequest, errors.New("rule text is required"))
			return
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if r.Method == http.MethodPost {
		if _, err := instructions.AddRule(instructions.ScopeGlobal, "", request.Text); err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
	}
	rules, err := listGlobalRules()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": rules})
}

func (a *customizationAPI) handleReorderRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		RuleIDs []int `json:"ruleIds"`
	}
	if err := decodeJSONBody(w, r, &request, 64<<10); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	a.mu.Lock()
	rules, err := reorderGlobalRules(request.RuleIDs)
	a.mu.Unlock()
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": rules})
}

func (a *customizationAPI) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		ID   int    `json:"id"`
		Text string `json:"text"`
	}
	if err := decodeJSONBody(w, r, &request, 256<<10); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	a.mu.Lock()
	rules, err := updateGlobalRule(request.ID, request.Text)
	a.mu.Unlock()
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": rules})
}

func (a *customizationAPI) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		ID int `json:"id"`
	}
	if err := decodeJSONBody(w, r, &request, 4096); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	a.mu.Lock()
	rules, err := deleteGlobalRule(request.ID)
	a.mu.Unlock()
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": rules})
}

func listGlobalRules() ([]apiCustomizationRule, error) {
	entries, err := instructions.ListRules(instructions.ScopeGlobal, "")
	if err != nil {
		return nil, err
	}
	rules := make([]apiCustomizationRule, 0, len(entries))
	for _, entry := range entries {
		rules = append(rules, apiCustomizationRule{ID: entry.Number, Text: entry.Text, Enabled: entry.Enabled})
	}
	return rules, nil
}

func reorderGlobalRules(ids []int) ([]apiCustomizationRule, error) {
	entries, err := instructions.ReorderRules(instructions.ScopeGlobal, "", ids)
	if err != nil {
		return nil, err
	}
	rules := make([]apiCustomizationRule, 0, len(entries))
	for _, entry := range entries {
		rules = append(rules, apiCustomizationRule{ID: entry.Number, Text: entry.Text, Enabled: entry.Enabled})
	}
	return rules, nil
}

func updateGlobalRule(id int, text string) ([]apiCustomizationRule, error) {
	text = strings.TrimSpace(text)
	if id <= 0 || text == "" {
		return nil, errors.New("rule id and text are required")
	}
	entries, err := instructions.ListRules(instructions.ScopeGlobal, "")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.Number == id {
			if err := os.WriteFile(entry.FilePath, []byte(text), 0o600); err != nil {
				return nil, err
			}
			return listGlobalRules()
		}
	}
	return nil, errors.New("rule not found")
}

func deleteGlobalRule(id int) ([]apiCustomizationRule, error) {
	if err := instructions.RemoveRule(instructions.ScopeGlobal, "", id); err != nil {
		return nil, err
	}
	return listGlobalRules()
}
