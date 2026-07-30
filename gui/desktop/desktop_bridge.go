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
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/project"
	toml "github.com/pelletier/go-toml/v2"
)

// DesktopBridge gives the Wails UI direct access to user-owned Solomon data.
// Vite middleware is only available to the browser development server, while a
// Wails WebView is served from wails.localhost.
type DesktopBridge struct{}

type desktopSidebarData struct {
	Projects        []desktopProject `json:"projects"`
	ReasoningEffort string           `json:"reasoningEffort"`
	UserName        string           `json:"userName"`
}

type desktopProject struct {
	Chats     []desktopChat `json:"chats"`
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Path      string        `json:"path"`
	ChatCount int           `json:"chatCount"`
	activity  time.Time
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
	return desktopSidebarData{Projects: projects, ReasoningEffort: loadDesktopReasoningEffort(), UserName: userName}, nil
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
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("read config.toml: %w", err)
	}
	cfg.ReasoningEffort = canonical
	if err := config.Save(cfg); err != nil {
		return "", fmt.Errorf("save config.toml: %w", err)
	}
	return canonical, nil
}

// RemoveProjectFromSidebar unregisters a project while keeping its on-disk
// Solomon data intact.
func (DesktopBridge) RemoveProjectFromSidebar(projectID string) error {
	return removeDesktopProject(projectID, false)
}

// RemoveProjectFromDisk unregisters a project and permanently deletes its
// Solomon project data, including chats.
func (DesktopBridge) RemoveProjectFromDisk(projectID string) error {
	return removeDesktopProject(projectID, true)
}

func (DesktopBridge) ProjectRemovalInfo(projectID string) (desktopProjectRemovalInfo, error) {
	mapPath, err := paths.ProjectsMapPath()
	if err != nil {
		return desktopProjectRemovalInfo{}, err
	}
	projectMap, err := project.LoadMap(mapPath)
	if err != nil {
		return desktopProjectRemovalInfo{}, err
	}
	projectPath := ""
	for registeredPath, registeredID := range projectMap {
		if registeredID == projectID {
			projectPath = registeredPath
			break
		}
	}
	if projectPath == "" {
		return desktopProjectRemovalInfo{}, fmt.Errorf("project is not registered")
	}
	absoluteProjectPath, err := filepath.Abs(filepath.Clean(projectPath))
	if err != nil {
		return desktopProjectRemovalInfo{}, err
	}
	dataPath, err := paths.ProjectRoot(projectID)
	if err != nil {
		return desktopProjectRemovalInfo{}, err
	}
	projectSize, err := desktopDirectorySize(absoluteProjectPath)
	if err != nil {
		return desktopProjectRemovalInfo{}, err
	}
	dataSize, err := desktopDirectorySize(dataPath)
	if err != nil {
		return desktopProjectRemovalInfo{}, err
	}
	return desktopProjectRemovalInfo{DataPath: dataPath, DataSizeBytes: dataSize, ProjectPath: absoluteProjectPath, ProjectSizeBytes: projectSize}, nil
}

// ProjectDirectoryEntries returns one directory level from a registered project.
// The relative path is constrained to the project root before it is read.
func (DesktopBridge) ProjectDirectoryEntries(projectID, relativePath string) ([]desktopProjectDirectoryEntry, error) {
	projectPath, err := desktopRegisteredProjectPath(projectID)
	if err != nil {
		return nil, err
	}
	root, err := filepath.Abs(filepath.Clean(projectPath))
	if err != nil {
		return nil, err
	}
	target := filepath.Clean(filepath.Join(root, relativePath))
	relativeTarget, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(relativeTarget) || relativeTarget == ".." || strings.HasPrefix(relativeTarget, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("invalid project directory")
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, err
	}
	result := make([]desktopProjectDirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		entryPath := entry.Name()
		if relativeTarget != "." {
			entryPath = filepath.Join(relativeTarget, entryPath)
		}
		result = append(result, desktopProjectDirectoryEntry{Name: entry.Name(), Path: entryPath, IsDirectory: entry.IsDir()})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDirectory != result[j].IsDirectory {
			return result[i].IsDirectory
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func desktopRegisteredProjectPath(projectID string) (string, error) {
	mapPath, err := paths.ProjectsMapPath()
	if err != nil {
		return "", err
	}
	projectMap, err := project.LoadMap(mapPath)
	if err != nil {
		return "", err
	}
	for projectPath, registeredID := range projectMap {
		if registeredID == projectID {
			return projectPath, nil
		}
	}
	return "", fmt.Errorf("project is not registered")
}

func desktopDirectorySize(directory string) (int64, error) {
	var size int64
	err := filepath.WalkDir(directory, func(_ string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		size += info.Size()
		return nil
	})
	if os.IsNotExist(err) || err != nil {
		return size, nil
	}
	return size, nil
}

func removeDesktopProject(projectID string, removeData bool) error {
	if len(projectID) != 64 {
		return fmt.Errorf("invalid project id")
	}
	for _, char := range projectID {
		if !(char >= '0' && char <= '9') && !(char >= 'a' && char <= 'f') {
			return fmt.Errorf("invalid project id")
		}
	}

	mapPath, err := paths.ProjectsMapPath()
	if err != nil {
		return err
	}
	projectMap, err := project.LoadMap(mapPath)
	if err != nil {
		return err
	}
	found := false
	registeredPaths := make([]string, 0, 1)
	for projectPath, registeredID := range projectMap {
		if registeredID == projectID {
			delete(projectMap, projectPath)
			found = true
			registeredPaths = append(registeredPaths, projectPath)
		}
	}
	if !found {
		return fmt.Errorf("project is not registered")
	}

	if removeData {
		for _, registeredPath := range registeredPaths {
			if err := removeDesktopProjectDirectory(registeredPath); err != nil {
				return err
			}
		}
		projectRoot, err := paths.ProjectRoot(projectID)
		if err != nil {
			return err
		}
		projectsDir, err := paths.ProjectsDir()
		if err != nil {
			return err
		}
		relativeRoot, err := filepath.Rel(projectsDir, projectRoot)
		if err != nil || relativeRoot != projectID {
			return fmt.Errorf("invalid project data path")
		}
		if err := os.RemoveAll(projectRoot); err != nil {
			return fmt.Errorf("remove project data: %w", err)
		}
	}
	return project.SaveMap(mapPath, projectMap)
}

func removeDesktopProjectDirectory(projectPath string) error {
	cleanPath, err := filepath.Abs(filepath.Clean(projectPath))
	if err != nil {
		return fmt.Errorf("resolve project path: %w", err)
	}
	if cleanPath == string(filepath.Separator) {
		return fmt.Errorf("refusing to remove filesystem root")
	}
	if home, err := os.UserHomeDir(); err == nil {
		cleanHome, _ := filepath.Abs(filepath.Clean(home))
		if cleanPath == cleanHome {
			return fmt.Errorf("refusing to remove home directory")
		}
	}
	if err := os.RemoveAll(cleanPath); err != nil {
		return fmt.Errorf("remove project directory: %w", err)
	}
	return nil
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
		project := desktopProject{ID: projectID, Name: desktopProjectDisplayName(projectPath), Path: projectPath, Chats: make([]desktopChat, 0, len(chats)), ChatCount: len(chats)}
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

func desktopProjectDisplayName(projectPath string) string {
	if home, err := os.UserHomeDir(); err == nil && filepath.Clean(projectPath) == filepath.Clean(home) {
		return "Home"
	}
	name := filepath.Base(projectPath)
	if name == "." || name == string(filepath.Separator) {
		return projectPath
	}
	return name
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

func loadDesktopReasoningEffort() string {
	configPath, err := paths.ConfigPath()
	if err != nil {
		return "none"
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "none"
	}
	var root struct {
		ReasoningEffort string `toml:"reasoning_effort"`
	}
	if toml.Unmarshal(data, &root) != nil {
		return "none"
	}
	canonical, err := config.ParseReasoningEffortToken(root.ReasoningEffort)
	if err != nil {
		return "none"
	}
	return canonical
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
