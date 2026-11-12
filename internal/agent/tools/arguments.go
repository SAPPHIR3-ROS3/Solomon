package tools

import (
	"encoding/json"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/validation"
)

func validateArguments(inv tooling.Invocation) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(inv.Args, &fields); err != nil || fields == nil {
		return validation.Argument(inv.Name, "arguments", "must be a valid JSON object")
	}
	if err := tooling.ValidateToolIntent(inv.Args); err != nil {
		return &validation.Issue{Kind: "arguments", Component: inv.Name, Field: "intent", Problem: "must be a non-empty string", Cause: err}
	}
	if !isInternalToolName(inv.Name) {
		return nil
	}
	required := ""
	switch inv.Name {
	case "readFile", "editFile":
		required = "path"
	case "shell":
		required = "command"
	case "webSearch":
		required = "query"
	case "fetchWeb":
		required = "url"
	}
	if required != "" {
		var value *string
		if json.Unmarshal(fields[required], &value) != nil || value == nil || strings.TrimSpace(*value) == "" {
			return validation.Argument(inv.Name, required, "must be a non-empty string")
		}
		if (required == "path" || required == "command") && strings.ContainsRune(*value, '\x00') {
			return validation.Argument(inv.Name, required, "must not contain NUL bytes")
		}
	}
	var positive []string
	switch inv.Name {
	case "shell", "find", "fetchWeb":
		positive = []string{"timeoutSeconds"}
	case "webSearch":
		positive = []string{"timeoutSeconds", "maxResults"}
	case "readFile", "editFile":
		positive = []string{"startLine", "endLine"}
	}
	for _, name := range positive {
		raw, exists := fields[name]
		if !exists {
			continue
		}
		var value *int
		if json.Unmarshal(raw, &value) != nil || value == nil || *value < 1 {
			return validation.Argument(inv.Name, name, "must be a positive integer when supplied")
		}
	}
	if inv.Name == "readFile" || inv.Name == "editFile" {
		var start, end int
		_ = json.Unmarshal(fields["startLine"], &start)
		_ = json.Unmarshal(fields["endLine"], &end)
		if start > 0 && end > 0 && start > end {
			return validation.Argument(inv.Name, "startLine/endLine", "startLine must not exceed endLine")
		}
	}
	if inv.Name == "editFile" {
		for _, name := range []string{"oldString", "newString", "renameTo"} {
			if raw, exists := fields[name]; exists {
				var value *string
				if json.Unmarshal(raw, &value) != nil || value == nil {
					return validation.Argument(inv.Name, name, "must be a string when supplied")
				}
			}
		}
	}
	if inv.Name == "editFile" {
		if raw, exists := fields["delete"]; exists {
			var value *bool
			if json.Unmarshal(raw, &value) != nil || value == nil {
				return validation.Argument(inv.Name, "delete", "must be a boolean when supplied")
			}
		}
	}
	if inv.Name == "webSearch" {
		if raw, exists := fields["engine"]; exists {
			var value *string
			if json.Unmarshal(raw, &value) != nil || value == nil {
				return validation.Argument(inv.Name, "engine", "must be a string when supplied")
			}
		}
		if raw, exists := fields["extras"]; exists {
			var extras map[string]any
			if json.Unmarshal(raw, &extras) != nil || extras == nil {
				return validation.Argument(inv.Name, "extras", "must be an object when supplied")
			}
		}
	}
	return nil
}
