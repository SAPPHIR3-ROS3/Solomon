package cloak

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type ToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

func (c *Client) CallTool(ctx context.Context, name string, args map[string]any, intent string) (map[string]any, error) {
	if c == nil {
		return nil, ErrUnavailable
	}
	if strings.TrimSpace(intent) == "" {
		return nil, fmt.Errorf("Cloak tool intent is required")
	}
	var spec *ToolSpec
	for _, candidate := range Tools() {
		if candidate.Name == name {
			copy := candidate
			spec = &copy
			break
		}
	}
	if spec == nil {
		return nil, fmt.Errorf("unknown Cloak tool %q", name)
	}
	if args == nil {
		args = map[string]any{}
	}
	props := spec.Parameters["properties"].(map[string]any)
	for key := range args {
		if _, ok := props[key]; !ok {
			return nil, fmt.Errorf("%s: unsupported parameter %s", name, key)
		}
	}
	for _, key := range spec.Parameters["required"].([]string) {
		if value, ok := args[key]; !ok || value == nil {
			return nil, fmt.Errorf("%s: %s is required", name, key)
		}
	}
	for key, value := range args {
		property := props[key].(map[string]any)
		kind := property["type"].(string)
		valid := false
		switch kind {
		case "string":
			_, valid = value.(string)
		case "boolean":
			_, valid = value.(bool)
		case "number", "integer":
			encoded, err := json.Marshal(value)
			if err == nil && len(encoded) > 0 && encoded[0] != '"' {
				var number json.Number
				if json.Unmarshal(encoded, &number) == nil {
					if kind == "integer" {
						_, err = number.Int64()
					} else {
						_, err = number.Float64()
					}
					valid = err == nil
				}
			}
		case "array":
			encoded, err := json.Marshal(value)
			if err == nil && len(encoded) > 0 {
				valid = encoded[0] == '['
			}
		case "object":
			_, valid = value.(map[string]any)
		}
		if !valid {
			return nil, fmt.Errorf("%s: %s must be %s", name, key, kind)
		}
	}
	var result map[string]any
	err := c.call(ctx, intent, "tool", map[string]any{"name": name, "parameters": args}, &result)
	return result, err
}
