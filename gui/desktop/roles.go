package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/roles"
)

type desktopSubagentScore struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value int    `json:"value"`
}

type desktopCharacteristic struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type desktopRolesTable struct {
	Catalog         []desktopCharacteristic `json:"catalog"`
	Characteristics []string                `json:"characteristics"`
	Max             int                     `json:"max"`
}

type desktopScorePatch struct {
	ID    string `json:"id"`
	Value int    `json:"value"`
}

func (DesktopBridge) CustomizationSubagents() ([]desktopCatalogItem, error) {
	return loadDesktopSubagents()
}

func (DesktopBridge) UpdateCustomizationSubagent(id string, detail string, scores []desktopScorePatch) ([]desktopCatalogItem, error) {
	index, err := desktopSubagentIndex(id)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("read config.toml: %w", err)
	}
	if index < 0 || index >= len(cfg.Roles.Subagent) {
		return nil, fmt.Errorf("subagent not found")
	}
	cfg.Roles.Subagent[index].Description = strings.TrimSpace(detail)
	if cfg.Roles.Subagent[index].Scores == nil {
		cfg.Roles.Subagent[index].Scores = map[string]int{}
	}
	for _, score := range scores {
		key := strings.TrimSpace(score.ID)
		if key == "" {
			continue
		}
		if !roles.IsKnownCharacteristic(key) {
			return nil, fmt.Errorf("unknown characteristic %q", key)
		}
		if err := roles.ValidateScoreValue(key, score.Value); err != nil {
			return nil, err
		}
		cfg.Roles.Subagent[index].Scores[key] = score.Value
	}
	if err := config.Save(cfg); err != nil {
		return nil, fmt.Errorf("save config.toml: %w", err)
	}
	return loadDesktopSubagents()
}

func (DesktopBridge) DeleteCustomizationSubagent(id string) ([]desktopCatalogItem, error) {
	index, err := desktopSubagentIndex(id)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("read config.toml: %w", err)
	}
	if index < 0 || index >= len(cfg.Roles.Subagent) {
		return nil, fmt.Errorf("subagent not found")
	}
	cfg.Roles.Subagent = append(cfg.Roles.Subagent[:index], cfg.Roles.Subagent[index+1:]...)
	if err := config.Save(cfg); err != nil {
		return nil, fmt.Errorf("save config.toml: %w", err)
	}
	return loadDesktopSubagents()
}

func (DesktopBridge) RolesTable() (desktopRolesTable, error) {
	return loadDesktopRolesTable()
}

func (DesktopBridge) SaveRolesTable(characteristics []string) (desktopRolesTable, error) {
	cleaned := make([]string, 0, len(characteristics))
	seen := map[string]bool{}
	for _, item := range characteristics {
		id := strings.TrimSpace(item)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		cleaned = append(cleaned, id)
	}
	if err := roles.ValidateTableCharacteristics(cleaned); err != nil {
		return desktopRolesTable{}, err
	}
	cfg, err := config.Load()
	if err != nil {
		return desktopRolesTable{}, fmt.Errorf("read config.toml: %w", err)
	}
	cfg.Roles.Table.Characteristics = cleaned
	if err := config.Save(cfg); err != nil {
		return desktopRolesTable{}, fmt.Errorf("save config.toml: %w", err)
	}
	return loadDesktopRolesTable()
}

func desktopSubagentIndex(id string) (int, error) {
	id = strings.TrimSpace(id)
	offset := strings.LastIndex(id, ":")
	if offset < 0 || offset == len(id)-1 {
		return -1, fmt.Errorf("invalid subagent id")
	}
	index, err := strconv.Atoi(id[offset+1:])
	if err != nil || index < 0 {
		return -1, fmt.Errorf("invalid subagent id")
	}
	return index, nil
}

func loadDesktopSubagents() ([]desktopCatalogItem, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("read config.toml: %w", err)
	}
	items := make([]desktopCatalogItem, 0, len(cfg.Roles.Subagent))
	for index, role := range cfg.Roles.Subagent {
		provider := strings.TrimSpace(role.Provider)
		model := strings.TrimSpace(role.Model)
		title := model
		if title == "" {
			title = fmt.Sprintf("subagent-%d", index+1)
		}
		scoreFields := make([]desktopSubagentScore, 0, len(cfg.Roles.Table.Characteristics))
		for _, characteristic := range cfg.Roles.Table.Characteristics {
			value := 0
			if role.Scores != nil {
				value = role.Scores[characteristic]
			}
			scoreFields = append(scoreFields, desktopSubagentScore{
				ID:    characteristic,
				Label: roles.CharacteristicLabel(characteristic),
				Value: value,
			})
		}
		items = append(items, desktopCatalogItem{
			Badge:  provider,
			Detail: strings.TrimSpace(role.Description),
			ID:     fmt.Sprintf("%s:%s:%d", provider, model, index),
			Scores: scoreFields,
			Title:  title,
		})
	}
	return items, nil
}

func loadDesktopRolesTable() (desktopRolesTable, error) {
	catalog := make([]desktopCharacteristic, 0, len(roles.AllCharacteristics))
	for _, id := range roles.AllCharacteristics {
		catalog = append(catalog, desktopCharacteristic{ID: id, Label: roles.CharacteristicLabel(id)})
	}
	cfg, err := config.Load()
	if err != nil {
		return desktopRolesTable{}, fmt.Errorf("read config.toml: %w", err)
	}
	selected := append([]string{}, cfg.Roles.Table.Characteristics...)
	return desktopRolesTable{Catalog: catalog, Characteristics: selected, Max: roles.MaxTableCharacteristics}, nil
}

type desktopPromptTemplate struct {
	Content  string `json:"content"`
	ID       string `json:"id"`
	Modified bool   `json:"modified"`
	Title    string `json:"title"`
}

func (DesktopBridge) CustomizationPromptTemplates() ([]desktopCatalogItem, error) {
	return loadDesktopPromptTemplates()
}

func (DesktopBridge) CustomizationPromptTemplate(id string) (desktopPromptTemplate, error) {
	return loadDesktopPromptTemplate(id)
}

func (DesktopBridge) UpdateCustomizationPromptTemplate(id string, content string) (desktopPromptTemplate, error) {
	id = strings.TrimSpace(id)
	if err := prompt.AcceptTemplateContent(id, content); err != nil {
		return desktopPromptTemplate{}, err
	}
	return loadDesktopPromptTemplate(id)
}

func (DesktopBridge) ResetCustomizationPromptTemplate(id string) (desktopPromptTemplate, error) {
	id = strings.TrimSpace(id)
	if _, err := prompt.ResetTemplateToDefault(id); err != nil {
		return desktopPromptTemplate{}, err
	}
	return loadDesktopPromptTemplate(id)
}

func loadDesktopPromptTemplates() ([]desktopCatalogItem, error) {
	dir, err := paths.PromptTemplatesDir()
	if err != nil {
		return nil, err
	}
	names := prompt.TemplateNames()
	sort.Strings(names)
	items := make([]desktopCatalogItem, 0, len(names))
	for _, name := range names {
		title := name + ".tmpl"
		item := desktopCatalogItem{ID: name, Title: title, Detail: desktopPromptTemplateDetail(name)}
		if _, err := os.Stat(filepath.Join(dir, title)); os.IsNotExist(err) {
			item.Badge = "Missing"
		} else if modified, err := prompt.TemplateDiffersFromEmbedded(name); err == nil && modified {
			item.Badge = "Modified"
		}
		items = append(items, item)
	}
	return items, nil
}

func loadDesktopPromptTemplate(id string) (desktopPromptTemplate, error) {
	id = strings.TrimSpace(id)
	if _, ok := prompt.EmbeddedTemplate(id); !ok {
		return desktopPromptTemplate{}, fmt.Errorf("unknown prompt template %q", id)
	}
	content, err := prompt.TemplateContent(id)
	if err != nil {
		return desktopPromptTemplate{}, err
	}
	modified, err := prompt.TemplateDiffersFromEmbedded(id)
	if err != nil {
		return desktopPromptTemplate{}, err
	}
	return desktopPromptTemplate{ID: id, Title: id + ".tmpl", Content: content, Modified: modified}, nil
}

func desktopPromptTemplateDetail(name string) string {
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
