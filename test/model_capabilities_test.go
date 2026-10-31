package test

import (
	"context"
	"encoding/json"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelreasoning"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelsapi"
	"github.com/openai/openai-go/v2"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestModelReasoningCapabilities(t *testing.T) {
	cases := []struct {
		name, raw string
		known     bool
		levels    []string
	}{
		{"unknown", `{}`, false, []string{}},
		{"generic support is not a level list", `{"supported_parameters":["reasoning"]}`, false, []string{}},
		{"unsupported", `{"supported_parameters":["temperature"]}`, true, []string{}},
		{"codex", `{"supported_reasoning_levels":[{"effort":"high"},{"effort":"low"},{"effort":"high"}]}`, true, []string{"low", "high"}},
		{"codex absent", `{"supported_reasoning_levels":null}`, false, []string{}},
		{"codex unsupported", `{"supported_reasoning_levels":[]}`, true, []string{}},
		{"anthropic", `{"capabilities":{"effort":{"supported":true,"low":{"supported":true},"high":{"supported":true},"max":{"supported":false}}}}`, true, []string{"low", "high"}},
		{"anthropic unsupported", `{"capabilities":{"effort":{"supported":false}}}`, true, []string{}},
		{"openrouter", `{"reasoning":{"supported_efforts":["high","minimal","low"],"mandatory":true}}`, true, []string{"minimal", "low", "high"}},
		{"openrouter mandatory", `{"reasoning":{"supported_efforts":["none","high"],"mandatory":true}}`, true, []string{"high"}},
		{"openrouter unrestricted", `{"reasoning":{"supported_efforts":null}}`, true, []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}},
		{"cursor", `{"parameters":[{"id":"reasoning","values":[{"value":"low"},{"value":"xhigh"}]}]}`, true, []string{"low", "xhigh"}},
		{"unrecognized", `{"supported_reasoning_levels":["future-level"]}`, false, []string{}},
		{"invalid", `{"supported_reasoning_levels":true}`, false, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := modelreasoning.Parse([]byte(tc.raw))
			if c.Known != tc.known || !reflect.DeepEqual(c.Levels, tc.levels) {
				t.Fatalf("capability = %+v", c)
			}
		})
	}
}
func TestModelReasoningResolutionAndIsolation(t *testing.T) {
	c := modelreasoning.Capability{Known: true, Levels: []string{"low", "high"}, Default: "high"}
	if c.Resolve("max") != "high" || c.Resolve("low") != "low" {
		t.Fatal("invalid capability resolution")
	}
	if (modelreasoning.Capability{}).Resolve("") != "medium" {
		t.Fatal("unknown fallback changed")
	}
	if (modelreasoning.Capability{Known: true}).Resolve("high") != "" {
		t.Fatal("unsupported model gained an effort")
	}
	modelreasoning.Store(t.Name()+"/a", "same", c)
	modelreasoning.Store(t.Name()+"/b", "same", modelreasoning.Capability{Known: true, Levels: []string{"low"}})
	first := modelreasoning.Lookup(t.Name()+"/a", "same")
	first.Levels[0] = "max"
	if modelreasoning.Lookup(t.Name()+"/a", "same").Levels[0] != "low" || len(modelreasoning.Lookup(t.Name()+"/b", "same").Levels) != 1 {
		t.Fatal("cache leaked across provider or caller")
	}
}
func TestModelReasoningAutomaticDiscovery(t *testing.T) {
	fixture := `{"data":[{"id":"test-model","reasoning":{"supported_efforts":["low","high"]}}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(fixture)) }))
	defer server.Close()
	ids, err := modelsapi.List(server.URL, "test")
	if err != nil || len(ids) != 1 {
		t.Fatalf("models = %v, %v", ids, err)
	}
	if c := modelreasoning.Lookup(server.URL, ids[0]); !c.Known || len(c.Levels) != 2 {
		t.Fatalf("metadata discarded: %+v", c)
	}
}
func TestModelReasoningAnthropicDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"id": "claude-test", "type": "model", "display_name": "Test", "created_at": "2026-01-01T00:00:00Z", "capabilities": map[string]any{"effort": map[string]any{"supported": true, "high": map[string]any{"supported": true}}}}}, "has_more": false})
	}))
	defer server.Close()
	_, err := modelsapi.ListAnthropicWithClient(context.Background(), server.URL, "key", false, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	c := modelreasoning.Lookup(server.URL, "claude-test")
	if !c.Known || len(c.Levels) != 1 || c.Levels[0] != "high" {
		t.Fatalf("SDK dropped capabilities: %+v", c)
	}
}

func TestModelReasoningWireCapabilities(t *testing.T) {
	base := "https://" + t.Name() + ".invalid"
	cfg := config.EmptyRoot()
	cfg.Providers = map[string]*config.Provider{"test": {Name: "test", BaseURL: base}}
	cfg.Current = config.Current{Provider: "test", Model: "limited"}
	cfg.ReasoningEffort = "max"
	modelreasoning.Store(base, "limited", modelreasoning.Capability{Known: true, Levels: []string{"minimal", "high"}, Default: "high"})
	params := openai.ChatCompletionNewParams{}
	llm.ApplyChatReasoning(cfg, &params, false)
	if params.ReasoningEffort != "high" {
		t.Fatalf("invalid effort reached provider: %s", params.ReasoningEffort)
	}
	llm.ApplyChatReasoningWithEffort(cfg, &params, false, "minimal")
	if params.ReasoningEffort != "minimal" {
		t.Fatal("minimal was discarded")
	}
	cfg.Current.Model = "unsupported"
	modelreasoning.Store(base, "unsupported", modelreasoning.Capability{Known: true, Levels: []string{}})
	params = openai.ChatCompletionNewParams{}
	llm.ApplyChatReasoning(cfg, &params, false)
	if params.ReasoningEffort != "" {
		t.Fatal("reasoning sent to unsupported model")
	}
}
