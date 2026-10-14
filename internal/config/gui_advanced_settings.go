package config

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

func guiAdvancedSpecs() map[string]SettingSpec {
	all := SettingsSpecs()
	specs := map[string]SettingSpec{}
	for _, key := range []string{"compaction_threshold_tokens", "subagent_timeout_minutes", "research_max_rounds", "research_max_urls_per_round", "research_max_content_chars"} {
		specs[key] = all[key]
	}
	specs["server_port"] = SettingSpec{Type: "integer", Min: 1, Max: 65535}
	specs["tool_output.max_bytes"] = SettingSpec{Type: "integer", Min: 1, Max: 1048576}
	specs["tool_output.max_lines"] = SettingSpec{Type: "integer", Min: 1, Max: 1048576}
	return specs
}

func validateGUIAdvancedSettings(patch map[string]json.RawMessage) error {
	specs := guiAdvancedSpecs()
	for key, raw := range patch {
		if key == "server.dev_mode" {
			var enabled bool
			if string(raw) == "null" {
				return fmt.Errorf("server.dev_mode requires a boolean")
			}
			if err := json.Unmarshal(raw, &enabled); err != nil {
				return fmt.Errorf("server.dev_mode requires a boolean")
			}
			continue
		}
		if key == "server.dev_directory" {
			var directory string
			if string(raw) == "null" {
				return fmt.Errorf("server.dev_directory requires a string")
			}
			if err := json.Unmarshal(raw, &directory); err != nil {
				return fmt.Errorf("server.dev_directory requires a string")
			}
			directory = strings.TrimSpace(directory)
			if len(directory) > 256 || strings.ContainsAny(directory, "\r\n\x00") {
				return fmt.Errorf("GUI source directory must be a single-line path of at most 256 bytes")
			}
			if directory != "" {
				if !filepath.IsAbs(directory) && !strings.HasPrefix(directory, "~/") {
					return fmt.Errorf("GUI source directory must be an absolute path or start with ~/")
				}
				if _, err := ValidateDevDirectory(directory); err != nil {
					return err
				}
			}
			continue
		}
		if key == "log_level" {
			var level string
			if err := json.Unmarshal(raw, &level); err != nil {
				return fmt.Errorf("log_level requires a string")
			}
			if _, err := logging.ParseLevel(level); err != nil {
				return err
			}
			continue
		}
		spec, ok := specs[key]
		if !ok {
			return fmt.Errorf("unsupported advanced preference %q", key)
		}
		var number int64
		if err := json.Unmarshal(raw, &number); err != nil || string(raw) == "null" || number < spec.Min || number > spec.Max {
			return fmt.Errorf("%s requires an integer between %d and %d", key, spec.Min, spec.Max)
		}
	}
	return nil
}

func guiAdvancedValues(root *Root) map[string]any {
	level := strings.ToLower(strings.TrimSpace(root.LogLevel))
	if _, err := logging.ParseLevel(level); err != nil {
		level = "info"
	}
	maxBytes, maxLines := root.ToolOutput.MaxBytes, root.ToolOutput.MaxLines
	if maxBytes <= 0 {
		maxBytes = DefaultToolOutputMaxBytes
	}
	if maxLines <= 0 {
		maxLines = DefaultToolOutputMaxLines
	}
	return map[string]any{
		"compaction_threshold_tokens": EffectiveCompactionThresholdTokens(root),
		"subagent_timeout_minutes":    SubagentTimeout(root),
		"log_level":                   level,
		"server_port":                 root.EffectiveServerPort(),
		"server.dev_mode":             root.Server.DevMode != nil && *root.Server.DevMode,
		"server.dev_directory":        root.Server.DevDirectory,
		"research_max_rounds":         EffectiveResearchMaxRounds(root),
		"research_max_urls_per_round": EffectiveResearchMaxURLsPerRound(root),
		"research_max_content_chars":  EffectiveResearchMaxContentChars(root),
		"tool_output.max_bytes":       maxBytes,
		"tool_output.max_lines":       maxLines,
	}
}
