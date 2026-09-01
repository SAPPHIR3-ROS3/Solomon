package test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go/v2"
)

func TestValidateToolIntent(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		ok   bool
	}{
		{name: "string", raw: `{"intent":"inspect the file"}`, ok: true},
		{name: "whitespace is trimmed", raw: `{"intent":"  inspect the file  "}`, ok: true},
		{name: "missing", raw: `{"path":"main.go"}`},
		{name: "empty", raw: `{"intent":"  "}`},
		{name: "non string", raw: `{"intent":true}`},
		{name: "empty arguments", raw: `{}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tooling.ValidateToolIntent([]byte(tc.raw))
			if tc.ok {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if !errors.Is(err, tooling.ErrMissingToolIntent) {
				t.Fatalf("want ErrMissingToolIntent, got %v", err)
			}
		})
	}
	if got := tooling.ToolIntentDisplay([]byte(`{"query":"x"}`)); got != tooling.MissingToolIntentDisplay {
		t.Fatalf("missing display=%q", got)
	}
}

func TestNativeToolSchemasRequireIntent(t *testing.T) {
	for _, mode := range []string{"agent", "chat"} {
		params, err := agenttools.NativeToolParams(mode)
		if err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		assertIntentInNativeSchemas(t, mode, params)
	}
	assertIntentInNativeSchemas(t, "planning", agenttools.PlanningNativeToolParams())
}

func assertIntentInNativeSchemas(t *testing.T, label string, params []openai.ChatCompletionToolUnionParam) {
	t.Helper()
	for _, tool := range params {
		if tool.OfFunction == nil {
			continue
		}
		var schema map[string]any
		raw, err := json.Marshal(tool.OfFunction.Function.Parameters)
		if err != nil || json.Unmarshal(raw, &schema) != nil {
			t.Fatalf("%s/%s: invalid schema: %s", label, tool.OfFunction.Function.Name, raw)
		}
		properties, _ := schema["properties"].(map[string]any)
		if _, ok := properties["intent"]; !ok {
			t.Fatalf("%s/%s: intent property missing: %s", label, tool.OfFunction.Function.Name, raw)
		}
		if !requiredContainsIntent(schema["required"]) {
			t.Fatalf("%s/%s: intent is not required: %s", label, tool.OfFunction.Function.Name, raw)
		}
	}
}

func requiredContainsIntent(raw any) bool {
	values, ok := raw.([]any)
	if !ok {
		return false
	}
	for _, value := range values {
		if value == "intent" {
			return true
		}
	}
	return false
}

func TestMCPToolSchemaRequiresIntent(t *testing.T) {
	tool, err := mcp.AdaptTool("server", &sdkmcp.Tool{
		Name:        "search",
		Description: "Search things",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"q": map[string]any{"type": "string"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	properties, _ := tool.Schema["properties"].(map[string]any)
	if _, ok := properties["intent"]; !ok || !requiredContainsIntent(tool.Schema["required"]) {
		t.Fatalf("MCP schema does not require intent: %#v", tool.Schema)
	}
}

func TestLegacyToolCallRequiresIntent(t *testing.T) {
	_, err := tooling.ParseToolCallsBlock(`<tool_calls><tool name="readFile"><args>{"path":"main.go"}</args></tool></tool_calls>`)
	if !errors.Is(err, tooling.ErrMissingToolIntent) {
		t.Fatalf("want missing intent, got %v", err)
	}

	invs, err := tooling.ParseToolCallsBlock(`<tool_calls><tool name="readFile"><args>{"path":"main.go","intent":"inspect source"}</args></tool></tool_calls>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(invs) != 1 || !strings.Contains(string(invs[0].Args), `"intent":"inspect source"`) {
		t.Fatalf("unexpected invocation: %+v", invs)
	}
}

func TestToolExecRequiresIntent(t *testing.T) {
	_, err := agenttools.Exec(context.Background(), &agenttools.Env{}, "agent", tooling.Invocation{
		Name: "searchTools",
		Args: json.RawMessage(`{"query":"shell"}`),
	})
	if !errors.Is(err, tooling.ErrMissingToolIntent) {
		t.Fatalf("want missing intent, got %v", err)
	}
}
