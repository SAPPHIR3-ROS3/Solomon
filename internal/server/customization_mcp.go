package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

func (a *customizationAPI) handleSkills(w http.ResponseWriter, r *http.Request) {
	a.handleCatalogGET(w, r, "skills", readGlobalSkills)
}

func (a *customizationAPI) handleMCPs(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var request struct {
			ID       string `json:"id"`
			Disabled *bool  `json:"disabled"`
		}
		if err := decodeJSONBody(w, r, &request, 4096); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		if request.Disabled == nil {
			writeAPIError(w, http.StatusBadRequest, errors.New("disabled is required"))
			return
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if err := setMCPDisabled(request.ID, *request.Disabled); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		items, err := readMCPs()
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mcps": items})
		return
	}
	a.handleCatalogGET(w, r, "mcps", readMCPs)
}

// Preserve credentials and extension fields when toggling a server.
func setMCPDisabled(id string, disabled bool) error {
	path, err := paths.MCPConfigPath()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(b, &document); err != nil {
		return err
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(document["mcpServers"], &servers); err != nil {
		return err
	}
	raw, ok := servers[id]
	if !ok {
		return errors.New("MCP server not found")
	}
	var server map[string]json.RawMessage
	if err := json.Unmarshal(raw, &server); err != nil {
		return err
	}
	if server == nil {
		return errors.New("invalid MCP server object")
	}
	server["disabled"], err = json.Marshal(disabled)
	if err != nil {
		return err
	}
	servers[id], err = json.Marshal(server)
	if err != nil {
		return err
	}
	document["mcpServers"], err = json.Marshal(servers)
	if err != nil {
		return err
	}
	b, err = json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".mcp-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(append(b, '\n')); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), path)
}

func readGlobalSkills() ([]apiCustomizationItem, error) {
	path, err := paths.SkillsRegistryPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []apiCustomizationItem{}, nil
	}
	if err != nil {
		return nil, err
	}
	var document struct {
		Global map[string]struct {
			Name        string `json:"name"`
			Disabled    bool   `json:"disabled"`
			FrontMatter struct {
				Description string `json:"description"`
			} `json:"front_matter"`
		} `json:"global"`
	}
	if err := json.Unmarshal(b, &document); err != nil {
		return nil, err
	}
	items := make([]apiCustomizationItem, 0, len(document.Global))
	for id, value := range document.Global {
		title := strings.TrimSpace(value.Name)
		if title == "" {
			title = id
		}
		items = append(items, apiCustomizationItem{Detail: strings.TrimSpace(value.FrontMatter.Description), Disabled: value.Disabled, ID: id, Title: title})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Title < items[j].Title })
	return items, nil
}

func readMCPs() ([]apiCustomizationItem, error) {
	path, err := paths.MCPConfigPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []apiCustomizationItem{}, nil
	}
	if err != nil {
		return nil, err
	}
	var document struct {
		Servers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(b, &document); err != nil {
		return nil, err
	}
	items := make([]apiCustomizationItem, 0, len(document.Servers))
	for id, raw := range document.Servers {
		var server struct {
			Disabled bool   `json:"disabled"`
			Command  string `json:"command"`
			Type     string `json:"type"`
			URL      string `json:"url"`
		}
		_ = json.Unmarshal(raw, &server)
		detail := strings.TrimSpace(server.URL)
		if detail == "" {
			detail = strings.TrimSpace(server.Command)
		}
		if detail == "" {
			detail = strings.TrimSpace(server.Type)
		}
		items = append(items, apiCustomizationItem{Detail: detail, ID: id, Title: id, Disabled: server.Disabled})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Title < items[j].Title })
	return items, nil
}
