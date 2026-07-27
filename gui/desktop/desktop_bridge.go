package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/instructions"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	toml "github.com/pelletier/go-toml/v2"
)

// DesktopBridge gives the Wails UI direct access to user-owned Solomon data.
// Vite middleware is only available to the browser development server, while a
// Wails WebView is served from wails.localhost.
type DesktopBridge struct{}

type desktopSidebarData struct {
	Projects []desktopProject `json:"projects"`
	UserName string           `json:"userName"`
}

type desktopProject struct {
	Chats     []desktopChat `json:"chats"`
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Path      string        `json:"path"`
	ChatCount int           `json:"chatCount"`
	activity  time.Time
}

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
	// root TOML field directly; config.Load is still used for writes.
	userName := loadDesktopUserName()
	return desktopSidebarData{Projects: projects, UserName: userName}, nil
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

func loadDesktopProjects() ([]desktopProject, error) {
	mapPath, err := paths.ProjectsMapPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(mapPath)
	if os.IsNotExist(err) {
		return []desktopProject{}, nil
	}
	if err != nil {
		return nil, err
	}
	var projectIDs map[string]string
	if err := json.Unmarshal(data, &projectIDs); err != nil {
		return nil, fmt.Errorf("read projectsId.json: %w", err)
	}

	projects := make([]desktopProject, 0, len(projectIDs))
	for projectPath, projectID := range projectIDs {
		if strings.TrimSpace(projectPath) == "" || strings.TrimSpace(projectID) == "" {
			continue
		}
		chats, err := chatstore.ListRecent(projectID, 10_000)
		if err != nil {
			return nil, err
		}
		project := desktopProject{ID: projectID, Name: filepath.Base(projectPath), Path: projectPath, Chats: make([]desktopChat, 0, len(chats)), ChatCount: len(chats)}
		if project.Name == "." || project.Name == string(filepath.Separator) {
			project.Name = projectPath
		}
		for _, chat := range chats {
			project.Chats = append(project.Chats, desktopChat{ID: chat.ID, LastMessageAt: chat.LastMessageAt.UTC().Format(time.RFC3339), Title: chat.Title})
			if chat.LastMessageAt.After(project.activity) {
				project.activity = chat.LastMessageAt
			}
		}
		if project.activity.IsZero() {
			if projectRoot, err := paths.ProjectRoot(projectID); err == nil {
				if info, err := os.Stat(projectRoot); err == nil {
					project.activity = info.ModTime()
				}
			}
		}
		projects = append(projects, project)
	}
	sort.Slice(projects, func(i, j int) bool {
		if projects[i].activity.Equal(projects[j].activity) {
			return projects[i].Name < projects[j].Name
		}
		return projects[i].activity.After(projects[j].activity)
	})
	return projects, nil
}

func loadDesktopUserName() string {
	configPath, err := paths.ConfigPath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	var root struct {
		UserName string `toml:"user_name"`
	}
	if toml.Unmarshal(data, &root) != nil {
		return ""
	}
	return strings.TrimSpace(root.UserName)
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
