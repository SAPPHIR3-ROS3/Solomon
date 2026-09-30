package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

// SettingSpec is the explicit allowlist for model-accessible, non-secret settings.
type SettingSpec struct {
	Field string   `json:"-"`
	Type  string   `json:"type"`
	Min   int64    `json:"min,omitempty"`
	Max   int64    `json:"max,omitempty"`
	Enum  []string `json:"enum,omitempty"`
}

func SettingsSpecs() map[string]SettingSpec {
	return map[string]SettingSpec{
		"user_name":                   {Field: "UserName", Type: "string"},
		"response_language":           {Field: "ResponseLanguage", Type: "string"},
		"reasoning_effort":            {Field: "ReasoningEffort", Type: "string", Enum: []string{"none", "low", "medium", "high", "xhigh", "max"}},
		"subagent_reasoning_effort":   {Field: "SubagentReasoningEffort", Type: "string", Enum: []string{"none", "low", "medium", "high", "xhigh", "max"}},
		"show_thinking":               {Field: "ShowThinking", Type: "boolean"},
		"show_usage_stats":            {Field: "ShowUsageStats", Type: "boolean"},
		"fast_mode":                   {Field: "FastMode", Type: "boolean"},
		"anonymize":                   {Field: "Anonymize", Type: "boolean"},
		"autoupdate":                  {Field: "AutoUpdate", Type: "boolean"},
		"subagent_timeout_minutes":    {Field: "SubagentTimeoutMinutes", Type: "integer", Min: 1, Max: 180},
		"max_response_tokens":         {Field: "MaxResponseTokens", Type: "integer", Min: 1, Max: 1048576},
		"compaction_threshold_tokens": {Field: "CompactionThresholdTokens", Type: "integer", Min: MinCompactionThresholdTokens, Max: 1048576},
		"research_max_rounds":         {Field: "ResearchMaxRounds", Type: "integer", Min: 1, Max: 100},
		"research_max_urls_per_round": {Field: "ResearchMaxURLsPerRound", Type: "integer", Min: 1, Max: 100},
		"research_max_content_chars":  {Field: "ResearchMaxContentChars", Type: "integer", Min: 1, Max: 1000000},
	}
}

func SettingsValues(root *Root) map[string]any {
	out := map[string]any{}
	if root == nil {
		return out
	}
	for name, spec := range SettingsSpecs() {
		v := reflect.ValueOf(root).Elem().FieldByName(spec.Field)
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				out[name] = nil
				continue
			}
			v = v.Elem()
		}
		out[name] = v.Interface()
	}
	return out
}

// UpdateSetting compares under the config lock, validates before normalization,
// and logs only allowlisted values after successful atomic persistence.
func UpdateSetting(name string, value, expected json.RawMessage, audit map[string]any) (*Root, error) {
	spec, ok := SettingsSpecs()[name]
	if !ok {
		return nil, fmt.Errorf("unsupported setting %q; read settings for supported keys", name)
	}
	if len(expected) == 0 {
		return nil, fmt.Errorf("expected is required; read settings first")
	}
	var before any
	root, err := updateConfig(func(root *Root) error {
		before = SettingsValues(root)[name]
		encoded, _ := json.Marshal(before)
		var oldValue, expectedValue any
		if err := json.Unmarshal(expected, &expectedValue); err != nil {
			return err
		}
		_ = json.Unmarshal(encoded, &oldValue)
		if !reflect.DeepEqual(oldValue, expectedValue) {
			return fmt.Errorf("setting %s changed; read settings again before updating", name)
		}
		field := reflect.ValueOf(root).Elem().FieldByName(spec.Field)
		target := reflect.New(field.Type())
		if err := json.Unmarshal(value, target.Interface()); err != nil {
			return fmt.Errorf("%s requires %s", name, spec.Type)
		}
		if strings.TrimSpace(string(value)) == "null" || len(value) == 0 {
			return fmt.Errorf("value must not be null")
		}
		v := target.Elem()
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return fmt.Errorf("value must not be null")
			}
			v = v.Elem()
		}
		switch spec.Type {
		case "integer":
			if v.Int() < spec.Min || v.Int() > spec.Max {
				return fmt.Errorf("%s must be between %d and %d", name, spec.Min, spec.Max)
			}
		case "string":
			s := strings.TrimSpace(v.String())
			if s == "" || len(s) > 256 || strings.ContainsAny(s, "\r\n\x00") {
				return fmt.Errorf("%s requires a nonempty single-line string of at most 256 bytes", name)
			}
			if len(spec.Enum) > 0 {
				found := false
				for _, allowed := range spec.Enum {
					if s == allowed {
						found = true
					}
				}
				if !found {
					return fmt.Errorf("invalid %s; allowed values: %v", name, spec.Enum)
				}
			}
			v.SetString(s)
		}
		field.Set(target.Elem())
		return nil
	})
	if err != nil {
		return nil, err
	}
	params := map[string]any{"setting": name, "before": before, "after": SettingsValues(root)[name]}
	for _, key := range []string{"tool_call_id", "parent_tool_call_id", "intent"} {
		if v, ok := audit[key]; ok {
			params[key] = v
		}
	}
	logging.Log(logging.INFO_LOG_LEVEL, "setting updated", logging.LogOptions{Params: params})
	return root, nil
}

// ReloadSettings returns a new runtime snapshot of the allowlisted preferences.
// Preserve the active provider, model and services: settings cannot modify them.
// Existing requests keep their previous snapshot.
func ReloadSettings(current, persisted *Root) *Root {
	next := *current
	dest := reflect.ValueOf(&next).Elem()
	source := reflect.ValueOf(persisted).Elem()
	for _, spec := range SettingsSpecs() {
		dest.FieldByName(spec.Field).Set(source.FieldByName(spec.Field))
	}
	return &next
}
