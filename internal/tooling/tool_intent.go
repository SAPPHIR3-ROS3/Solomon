package tooling

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrMissingToolIntent = errors.New("missing tool intent")

const MissingToolIntentDisplay = "Intent missing"

const ToolIntentSchemaDescription = "Required brief phrase describing why this tool call is being made"

// SchemaWithRequiredToolIntent returns a shallow-cloned JSON Schema with the
// Solomon intent contract added. The intent property is reserved by Solomon
// and is always a required non-empty-string field at the dispatch boundary.
func SchemaWithRequiredToolIntent(schema map[string]any) map[string]any {
	out := make(map[string]any, len(schema)+3)
	for key, value := range schema {
		out[key] = value
	}
	if _, ok := out["type"]; !ok {
		out["type"] = "object"
	}

	properties := map[string]any{}
	if existing, ok := out["properties"].(map[string]any); ok {
		for key, value := range existing {
			properties[key] = value
		}
	}
	properties["intent"] = map[string]any{
		"type":        "string",
		"minLength":   1,
		"pattern":     `\S`,
		"description": ToolIntentSchemaDescription,
	}
	out["properties"] = properties

	required := make([]any, 0)
	switch values := out["required"].(type) {
	case []any:
		required = append(required, values...)
	case []string:
		for _, value := range values {
			required = append(required, value)
		}
	}
	hasIntent := false
	for _, value := range required {
		if name, ok := value.(string); ok && name == "intent" {
			hasIntent = true
			break
		}
	}
	if !hasIntent {
		required = append(required, "intent")
	}
	out["required"] = required
	return out
}

// ToolIntent returns the non-empty string intent carried by a JSON tool
// argument object. A non-string intent is not considered valid.
func ToolIntent(rawArgs json.RawMessage) (string, bool) {
	var args map[string]json.RawMessage
	if err := json.Unmarshal(rawArgs, &args); err != nil || args == nil {
		return "", false
	}
	rawIntent, ok := args["intent"]
	if !ok {
		return "", false
	}
	var intent string
	if err := json.Unmarshal(rawIntent, &intent); err != nil {
		return "", false
	}
	intent = strings.TrimSpace(intent)
	return intent, intent != ""
}

// ValidateToolIntent enforces the Solomon tool-call contract at the dispatch
// boundary. Invalid JSON is left to the argument decoder so callers retain
// the more useful malformed-arguments error.
func ValidateToolIntent(rawArgs json.RawMessage) error {
	if intent, ok := ToolIntent(rawArgs); ok && intent != "" {
		return nil
	}
	trimmed := strings.TrimSpace(string(rawArgs))
	var args map[string]json.RawMessage
	if trimmed == "" || json.Unmarshal(rawArgs, &args) == nil {
		return fmt.Errorf("%w: arguments.intent must be a non-empty string", ErrMissingToolIntent)
	}
	return nil
}

func ValidateInvocationIntents(invs []Invocation) error {
	for _, inv := range invs {
		if err := ValidateToolIntent(inv.Args); err != nil {
			return fmt.Errorf("%w: tool %q", err, strings.TrimSpace(inv.Name))
		}
	}
	return nil
}

func ToolIntentDisplay(rawArgs json.RawMessage) string {
	if intent, ok := ToolIntent(rawArgs); ok {
		return intent
	}
	return MissingToolIntentDisplay
}
