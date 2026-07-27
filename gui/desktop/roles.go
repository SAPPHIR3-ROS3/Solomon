package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
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
