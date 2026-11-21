package app

import (
	"encoding/json"
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/instructions"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	toml "github.com/pelletier/go-toml/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DesktopBridge gives the Wails UI direct access to user-owned Solomon data.
// Vite middleware is only available to the browser development server, while a
// Wails WebView is served from wails.localhost.
type DesktopBridge struct{}

type desktopSidebarData struct {
	FastMode        bool             `json:"fastMode"`
	Projects        []desktopProject `json:"projects"`
	ReasoningEffort string           `json:"reasoningEffort"`
	UserName        string           `json:"userName"`
}

type desktopProject struct {
	Chats      []desktopChat     `json:"chats"`
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Path       string            `json:"path"`
	ChatCount  int               `json:"chatCount"`
	TokenStats desktopTokenStats `json:"tokenStats"`
	activity   time.Time
}

type desktopTokenStats struct {
	User      int64 `json:"user"`
	Reasoning int64 `json:"reasoning"`
	Response  int64 `json:"response"`
	Total     int64 `json:"total"`
}

type desktopProjectRemovalInfo struct {
	DataPath         string `json:"dataPath"`
	DataSizeBytes    int64  `json:"dataSizeBytes"`
	ProjectPath      string `json:"projectPath"`
	ProjectSizeBytes int64  `json:"projectSizeBytes"`
}

type desktopProjectDirectoryEntry struct {
	IsDirectory bool   `json:"isDirectory"`
	Name        string `json:"name"`
	Path        string `json:"path"`
}

type desktopAtMentionSuggestion struct {
	IsDirectory bool   `json:"isDirectory"`
	Path        string `json:"path"`
	Tag         string `json:"tag"`
}

type desktopAtMentionEntry struct {
	IsDirectory bool   `json:"isDirectory"`
	Path        string `json:"path"`
}

var desktopAtMentionIndex = atmention.NewIndexCache()

type desktopChat struct {
	ID            string `json:"id"`
	LastMessageAt string `json:"lastMessageAt"`
	Title         string `json:"title"`
}

type desktopRule struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

type desktopCatalogItem struct {
	Badge  string                 `json:"badge,omitempty"`
	Detail string                 `json:"detail"`
	ID     string                 `json:"id"`
	Scores []desktopSubagentScore `json:"scores,omitempty"`
	Title  string                 `json:"title"`
}

func (DesktopBridge) ProjectSidebarData() (desktopSidebarData, error) {
	projects, err := loadDesktopProjects()
	if err != nil {
		return desktopSidebarData{}, err
	}
	// A malformed optional section must not hide projects and chats. Read the
	// root TOML fields directly; config.Load is still used for writes.
	settings := loadDesktopSettings()
	return desktopSidebarData{FastMode: settings.FastMode, Projects: projects, ReasoningEffort: settings.ReasoningEffort, UserName: settings.UserName}, nil
}

func (DesktopBridge) SaveUserName(userName string) (string, error) {
	userName = strings.TrimSpace(userName)
	if len(userName) > 120 {
		return "", fmt.Errorf("user name is too long")
	}
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("read config.toml: %w", err)
	}
	cfg.UserName = userName
	if err := config.Save(cfg); err != nil {
		return "", fmt.Errorf("save config.toml: %w", err)
	}
	return userName, nil
}

func (DesktopBridge) SaveReasoningEffort(effort string) (string, error) {
	canonical, err := config.ParseReasoningEffortToken(effort)
	if err != nil {
		return "", err
	}
	if _, err := config.UpdateReasoningEffort(canonical); err != nil {
		return "", fmt.Errorf("save config.toml: %w", err)
	}
	return canonical, nil
}

func (DesktopBridge) SaveFastMode(enabled bool) (bool, error) {
	cfg, err := config.Load()
	if err != nil {
		return false, fmt.Errorf("read config.toml: %w", err)
	}
	cfg.FastMode = &enabled
	if err := config.Save(cfg); err != nil {
		return false, fmt.Errorf("save config.toml: %w", err)
	}
	return enabled, nil
}

// RemoveProjectFromSidebar unregisters a project while keeping its on-disk
// Solomon data intact.
func (DesktopBridge) CustomizationRules() ([]desktopRule, error) {
	return loadDesktopRules()
}

func (DesktopBridge) ReorderCustomizationRules(ruleIDs []int) ([]desktopRule, error) {
	rules, err := loadDesktopRules()
	if err != nil {
		return nil, err
	}
	if len(rules) != len(ruleIDs) {
		return nil, fmt.Errorf("rule order does not match current rules")
	}
	byID := make(map[int]desktopRule, len(rules))
	for _, rule := range rules {
		byID[rule.ID] = rule
	}
	for _, id := range ruleIDs {
		if _, ok := byID[id]; !ok {
			return nil, fmt.Errorf("rule order does not match current rules")
		}
		delete(byID, id)
	}
	if len(byID) != 0 {
		return nil, fmt.Errorf("rule order contains duplicates")
	}

	rulesDir, err := paths.GlobalRulesDir()
	if err != nil {
		return nil, err
	}
	stamp := fmt.Sprintf(".reorder-%d", time.Now().UnixNano())
	for _, rule := range rules {
		from := filepath.Join(rulesDir, desktopRuleFileName(rule.ID))
		if err := os.Rename(from, from+stamp); err != nil {
			return nil, err
		}
	}
	for index, id := range ruleIDs {
		from := filepath.Join(rulesDir, desktopRuleFileName(id)+stamp)
		to := filepath.Join(rulesDir, desktopRuleFileName(index+1))
		if err := os.Rename(from, to); err != nil {
			return nil, err
		}
	}
	return loadDesktopRules()
}

func (DesktopBridge) UpdateCustomizationRule(ruleID int, text string) ([]desktopRule, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("rule text is empty")
	}
	if ruleID <= 0 {
		return nil, fmt.Errorf("invalid rule id")
	}
	rules, err := loadDesktopRules()
	if err != nil {
		return nil, err
	}
	found := false
	for _, rule := range rules {
		if rule.ID == ruleID {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("rule not found")
	}
	rulesDir, err := paths.GlobalRulesDir()
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(rulesDir, desktopRuleFileName(ruleID)), []byte(text), 0o600); err != nil {
		return nil, err
	}
	return loadDesktopRules()
}

func (DesktopBridge) DeleteCustomizationRule(ruleID int) ([]desktopRule, error) {
	if err := instructions.RemoveRule(instructions.ScopeGlobal, "", ruleID); err != nil {
		return nil, err
	}
	return loadDesktopRules()
}

func (DesktopBridge) CustomizationSkills() ([]desktopCatalogItem, error) {
	return loadDesktopSkills()
}

func (DesktopBridge) CustomizationMcps() ([]desktopCatalogItem, error) {
	return loadDesktopMcps()
}

type desktopSettings struct {
	FastMode        bool
	ReasoningEffort string
	UserName        string
}

func loadDesktopSettings() desktopSettings {
	settings := desktopSettings{FastMode: true, ReasoningEffort: "none"}
	configPath, err := paths.ConfigPath()
	if err != nil {
		return settings
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return settings
	}
	var root struct {
		FastMode        *bool  `toml:"fast_mode"`
		ReasoningEffort string `toml:"reasoning_effort"`
		UserName        string `toml:"user_name"`
	}
	if toml.Unmarshal(data, &root) != nil {
		return settings
	}
	canonical, err := config.ParseReasoningEffortToken(root.ReasoningEffort)
	if err == nil {
		settings.ReasoningEffort = canonical
	}
	if root.FastMode != nil {
		settings.FastMode = *root.FastMode
	}
	settings.UserName = strings.TrimSpace(root.UserName)
	return settings
}

func loadDesktopRules() ([]desktopRule, error) {
	rulesDir, err := paths.GlobalRulesDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(rulesDir)
	if os.IsNotExist(err) {
		return []desktopRule{}, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	rules := make([]desktopRule, 0, len(entries))
	for index, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		text, err := os.ReadFile(filepath.Join(rulesDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		if value := strings.TrimSpace(string(text)); value != "" {
			rules = append(rules, desktopRule{ID: index + 1, Text: value})
		}
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	return rules, nil
}

func loadDesktopSkills() ([]desktopCatalogItem, error) {
	registryPath, err := paths.SkillsRegistryPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(registryPath)
	if os.IsNotExist(err) {
		return []desktopCatalogItem{}, nil
	}
	if err != nil {
		return nil, err
	}
	var registry struct {
		Global map[string]struct {
			Name        string `json:"name"`
			FrontMatter struct {
				Description string `json:"description"`
			} `json:"front_matter"`
		} `json:"global"`
	}
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("read skills.json: %w", err)
	}
	items := make([]desktopCatalogItem, 0, len(registry.Global))
	for id, entry := range registry.Global {
		title := strings.TrimSpace(entry.Name)
		if title == "" {
			title = id
		}
		items = append(items, desktopCatalogItem{ID: id, Title: title, Detail: strings.TrimSpace(entry.FrontMatter.Description)})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Title < items[j].Title })
	return items, nil
}

func loadDesktopMcps() ([]desktopCatalogItem, error) {
	configPath, err := paths.MCPConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return []desktopCatalogItem{}, nil
	}
	if err != nil {
		return nil, err
	}
	var document struct {
		Servers map[string]struct {
			Command string `json:"command"`
			Type    string `json:"type"`
			URL     string `json:"url"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("read mcp.json: %w", err)
	}
	items := make([]desktopCatalogItem, 0, len(document.Servers))
	for id, server := range document.Servers {
		detail := strings.TrimSpace(server.URL)
		if detail == "" {
			detail = strings.TrimSpace(server.Command)
		}
		if detail == "" {
			detail = strings.TrimSpace(server.Type)
		}
		items = append(items, desktopCatalogItem{ID: id, Title: id, Detail: detail})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Title < items[j].Title })
	return items, nil
}

func desktopRuleFileName(id int) string {
	return fmt.Sprintf("rule_%02d.txt", id)
}
