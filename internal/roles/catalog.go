package roles

import (
	"fmt"
	"strings"
)

const MaxTableCharacteristics = 5

var AllCharacteristics = []string{
	"world_knowledge",
	"reasoning",
	"instruction_following",
	"science_and_math",
	"real_cost",
	"cost",
	"speed",
	"long_horizon",
	"agentic_capabilities",
	"taste",
	"consistency",
}

var characteristicLabels = map[string]string{
	"world_knowledge":       "world knowledge",
	"reasoning":             "reasoning",
	"instruction_following": "instruction following",
	"science_and_math":      "science and math",
	"real_cost":             "real cost",
	"cost":                  "cost",
	"speed":                 "speed",
	"long_horizon":          "long horizon",
	"agentic_capabilities":  "agentic capabilities",
	"taste":                 "taste",
	"consistency":           "consistency",
}

var characteristicSymbols = map[string]string{
	"world_knowledge":       "🌐",
	"reasoning":             "🧠",
	"instruction_following": "📋",
	"science_and_math":      "🔬",
	"real_cost":             "💰",
	"cost":                  "💵",
	"speed":                 "⚡",
	"long_horizon":          "⏳",
	"agentic_capabilities":  "🤖",
	"taste":                 "✦",
	"consistency":           "≡",
}

func CharacteristicSymbol(id string) string {
	if s, ok := characteristicSymbols[id]; ok {
		return s
	}
	return id
}

var characteristicAbbrevs = map[string]string{
	"world_knowledge":       "wk",
	"reasoning":             "R",
	"instruction_following": "IF",
	"science_and_math":      "sci",
	"real_cost":             "$",
	"cost":                  "c",
	"speed":                 "sp",
	"long_horizon":          "lh",
	"agentic_capabilities":  "Ag",
	"taste":                 "t",
	"consistency":           "co",
}

func CharacteristicAbbrev(id string) string {
	if a, ok := characteristicAbbrevs[id]; ok {
		return a
	}
	return id
}

func CharacteristicColumn(id string) string {
	return CharacteristicSymbol(id)
}

func CharacteristicLegend(ids []string) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		parts = append(parts, CharacteristicSymbol(id)+" "+CharacteristicLabel(id))
	}
	return strings.Join(parts, "  ")
}

func CharacteristicLabel(id string) string {
	if l, ok := characteristicLabels[id]; ok {
		return l
	}
	return id
}

func IsKnownCharacteristic(id string) bool {
	id = strings.TrimSpace(id)
	for _, c := range AllCharacteristics {
		if c == id {
			return true
		}
	}
	return false
}

func ValidateTableCharacteristics(ids []string) error {
	if len(ids) == 0 {
		return fmt.Errorf("roles.table: at least one characteristic required")
	}
	if len(ids) > MaxTableCharacteristics {
		return fmt.Errorf("roles.table: at most %d characteristics", MaxTableCharacteristics)
	}
	seen := map[string]bool{}
	for i, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			return fmt.Errorf("roles.table.characteristics[%d]: empty", i)
		}
		if !IsKnownCharacteristic(id) {
			return fmt.Errorf("roles.table.characteristics[%d]: unknown characteristic %q", i, id)
		}
		if seen[id] {
			return fmt.Errorf("roles.table.characteristics[%d]: duplicate %q", i, id)
		}
		seen[id] = true
	}
	return nil
}

func ValidateScoreValue(ch string, v int) error {
	if v < 0 || v > 100 {
		return fmt.Errorf("score %q must be 0-100, got %d", ch, v)
	}
	return nil
}
