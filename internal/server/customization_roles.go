package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/roles"
)

func (a *customizationAPI) handleRolesTable(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		table, err := loadRolesTable()
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"rolesTable": table})
		return
	}
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		Characteristics []string `json:"characteristics"`
	}
	if err := decodeJSONBody(w, r, &request, 8192); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	cleaned := make([]string, 0, len(request.Characteristics))
	seen := map[string]bool{}
	for _, value := range request.Characteristics {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			cleaned = append(cleaned, value)
		}
	}
	if err := roles.ValidateTableCharacteristics(cleaned); err != nil {
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
	cfg.Roles.Table.Characteristics = cleaned
	if err := config.Save(cfg); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	table, err := loadRolesTable()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rolesTable": table})
}

func loadRolesTable() (apiRolesTable, error) {
	catalog := make([]apiRolesCharacteristic, 0, len(roles.AllCharacteristics))
	for _, id := range roles.AllCharacteristics {
		catalog = append(catalog, apiRolesCharacteristic{ID: id, Label: roles.CharacteristicLabel(id)})
	}
	cfg, err := config.Load()
	if err != nil {
		return apiRolesTable{}, err
	}
	return apiRolesTable{Catalog: catalog, Characteristics: append([]string(nil), cfg.Roles.Table.Characteristics...), Max: roles.MaxTableCharacteristics}, nil
}
