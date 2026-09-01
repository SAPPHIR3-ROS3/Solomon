package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/instructions"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/roles"
)

// customizationAPI is deliberately daemon-owned. The mutex serializes file
// rewrites so a rule reorder cannot race a simultaneous edit from another
// client (for example a browser and the Wails window).
type customizationAPI struct {
	mu sync.Mutex
}

type apiCustomizationRule struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

type apiCustomizationScore struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value int    `json:"value"`
}

type apiCustomizationItem struct {
	Badge  string                  `json:"badge,omitempty"`
	Detail string                  `json:"detail"`
	ID     string                  `json:"id"`
	Scores []apiCustomizationScore `json:"scores,omitempty"`
	Title  string                  `json:"title"`
}

type apiRolesCharacteristic struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type apiRolesTable struct {
	Catalog         []apiRolesCharacteristic `json:"catalog"`
	Characteristics []string                 `json:"characteristics"`
	Max             int                      `json:"max"`
}

type apiPromptTemplate struct {
	Content  string `json:"content"`
	ID       string `json:"id"`
	Modified bool   `json:"modified"`
	Title    string `json:"title"`
}

func newCustomizationAPI() *customizationAPI { return &customizationAPI{} }

func (a *customizationAPI) handleRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	a.mu.Lock()
	rules, err := listGlobalRules()
	a.mu.Unlock()
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

func (a *customizationAPI) handleSkills(w http.ResponseWriter, r *http.Request) {
	a.handleCatalogGET(w, r, "skills", readGlobalSkills)
}

func (a *customizationAPI) handleMCPs(w http.ResponseWriter, r *http.Request) {
	a.handleCatalogGET(w, r, "mcps", readMCPs)
}

func (a *customizationAPI) handleSubagents(w http.ResponseWriter, r *http.Request) {
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

func (a *customizationAPI) handlePromptTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	items, err := loadPromptTemplates()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"promptTemplates": items})
}

func (a *customizationAPI) handlePromptTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	item, err := loadPromptTemplate(strings.TrimSpace(r.URL.Query().Get("id")))
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"promptTemplate": item})
}

func (a *customizationAPI) handleUpdatePromptTemplate(w http.ResponseWriter, r *http.Request) {
	a.handlePromptTemplateMutation(w, r, false)
}

func (a *customizationAPI) handleResetPromptTemplate(w http.ResponseWriter, r *http.Request) {
	a.handlePromptTemplateMutation(w, r, true)
}

func (a *customizationAPI) handlePromptTemplateMutation(w http.ResponseWriter, r *http.Request, reset bool) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		Content string `json:"content"`
		ID      string `json:"id"`
	}
	if err := decodeJSONBody(w, r, &request, 256<<10); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	var err error
	if reset {
		_, err = prompt.ResetTemplateToDefault(strings.TrimSpace(request.ID))
	} else {
		err = prompt.AcceptTemplateContent(strings.TrimSpace(request.ID), request.Content)
	}
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	item, err := loadPromptTemplate(request.ID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"promptTemplate": item})
}

func (a *customizationAPI) handleCatalogGET(w http.ResponseWriter, r *http.Request, key string, reader func() ([]apiCustomizationItem, error)) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	items, err := reader()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{key: items})
}

func listGlobalRules() ([]apiCustomizationRule, error) {
	entries, err := instructions.ListRules(instructions.ScopeGlobal, "")
	if err != nil {
		return nil, err
	}
	rules := make([]apiCustomizationRule, 0, len(entries))
	for _, entry := range entries {
		rules = append(rules, apiCustomizationRule{ID: entry.Number, Text: entry.Text})
	}
	return rules, nil
}

func reorderGlobalRules(ids []int) ([]apiCustomizationRule, error) {
	entries, err := instructions.ListRules(instructions.ScopeGlobal, "")
	if err != nil {
		return nil, err
	}
	current := make([]int, 0, len(entries))
	byID := make(map[int]string, len(entries))
	for _, entry := range entries {
		current = append(current, entry.Number)
		byID[entry.Number] = entry.FilePath
	}
	want := append([]int(nil), ids...)
	sort.Ints(current)
	sort.Ints(want)
	if len(current) != len(want) {
		return nil, errors.New("rule order does not match the current rules")
	}
	for index := range current {
		if current[index] != want[index] {
			return nil, errors.New("rule order does not match the current rules")
		}
	}
	directory, err := paths.GlobalRulesDir()
	if err != nil {
		return nil, err
	}
	suffix := fmt.Sprintf(".reorder-%d-%d", os.Getpid(), time.Now().UnixNano())
	temporary := make(map[int]string, len(entries))
	for _, entry := range entries {
		name := filepath.Join(directory, filepath.Base(entry.FilePath)+suffix)
		if err := os.Rename(entry.FilePath, name); err != nil {
			return nil, err
		}
		temporary[entry.Number] = name
	}
	for index, id := range ids {
		name := temporary[id]
		if name == "" {
			return nil, errors.New("missing temporary rule file")
		}
		if err := os.Rename(name, filepath.Join(directory, fmt.Sprintf("rule_%02d.txt", index+1))); err != nil {
			return nil, err
		}
	}
	return listGlobalRules()
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
		items = append(items, apiCustomizationItem{Detail: strings.TrimSpace(value.FrontMatter.Description), ID: id, Title: title})
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
			Command string `json:"command"`
			Type    string `json:"type"`
			URL     string `json:"url"`
		}
		_ = json.Unmarshal(raw, &server)
		detail := strings.TrimSpace(server.URL)
		if detail == "" {
			detail = strings.TrimSpace(server.Command)
		}
		if detail == "" {
			detail = strings.TrimSpace(server.Type)
		}
		items = append(items, apiCustomizationItem{Detail: detail, ID: id, Title: id})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Title < items[j].Title })
	return items, nil
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

func loadPromptTemplates() ([]apiCustomizationItem, error) {
	names := prompt.TemplateNames()
	sort.Strings(names)
	items := make([]apiCustomizationItem, 0, len(names))
	for _, name := range names {
		item := apiCustomizationItem{Detail: promptTemplateDetail(name), ID: name, Title: name + ".tmpl"}
		directory, err := prompt.TemplatesDir()
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(filepath.Join(directory, item.Title)); errors.Is(err, os.ErrNotExist) {
			item.Badge = "Missing"
		} else if modified, diffErr := prompt.TemplateDiffersFromEmbedded(name); diffErr == nil && modified {
			item.Badge = "Modified"
		}
		items = append(items, item)
	}
	return items, nil
}

func loadPromptTemplate(id string) (apiPromptTemplate, error) {
	id = strings.TrimSpace(id)
	if _, ok := prompt.EmbeddedTemplate(id); !ok {
		return apiPromptTemplate{}, errors.New("unknown prompt template")
	}
	content, err := prompt.TemplateContent(id)
	if err != nil {
		return apiPromptTemplate{}, err
	}
	modified, err := prompt.TemplateDiffersFromEmbedded(id)
	if err != nil {
		return apiPromptTemplate{}, err
	}
	return apiPromptTemplate{Content: content, ID: id, Modified: modified, Title: id + ".tmpl"}, nil
}

func promptTemplateDetail(name string) string {
	switch name {
	case "agent":
		return "Main agent-mode system prompt"
	case "atmention":
		return "At-mention workflow prompt"
	case "btw":
		return "Side-question (/btw) prompt"
	case "btw_system":
		return "Side-question (/btw) system prompt"
	case "chat":
		return "Chat-mode system prompt"
	case "images":
		return "Image workflow prompt"
	case "summarize":
		return "Conversation summarize prompt"
	case "summarize_system":
		return "Conversation summarize system prompt"
	case "title":
		return "Chat title generation prompt"
	default:
		return "System prompt template"
	}
}
