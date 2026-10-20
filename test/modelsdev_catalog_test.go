package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelsdev"
)

func TestLookupMatchesProviderNameAndExactModelID(t *testing.T) {
	catalog := modelsdev.Catalog{
		"openrouter": {
			ID:   "openrouter",
			Name: "OpenRouter",
			Models: map[string]modelsdev.Model{
				"anthropic/claude-test": {ID: "anthropic/claude-test"},
			},
		},
	}
	if _, ok := catalog.Lookup("Open Router", "", "anthropic/claude-test"); !ok {
		t.Fatal("expected provider name normalization to match")
	}
	if _, ok := catalog.Lookup("Open Router", "", "claude-test"); ok {
		t.Fatal("must not match a different model ID")
	}
}

func TestLookupMatchesChatGPTSubscriptionToOpenAI(t *testing.T) {
	catalog := modelsdev.Catalog{
		"openai": {
			ID:     "openai",
			Models: map[string]modelsdev.Model{"gpt-test": {ID: "gpt-test"}},
		},
	}
	if _, ok := catalog.Lookup("ChatGPT Sub", "https://chatgpt.com/backend-api/codex/v1", "gpt-test"); !ok {
		t.Fatal("expected ChatGPT subscription models to use OpenAI metadata")
	}
}

func TestLookupUsesOfficialProviderForCursorModels(t *testing.T) {
	catalog := modelsdev.Catalog{
		"anthropic": {
			Models: map[string]modelsdev.Model{"claude-test": {ID: "claude-test"}},
		},
		"openrouter": {
			Models: map[string]modelsdev.Model{"claude-test": {ID: "claude-test"}},
		},
	}
	if _, ok := catalog.Lookup("Cursor API", "", "claude-test"); !ok {
		t.Fatal("expected Cursor's Claude model to use Anthropic metadata")
	}
	if _, ok := catalog.Lookup("Cursor API", "", "composer-2.5"); ok {
		t.Fatal("Cursor-native models must not be attributed to an unrelated provider")
	}
}

func TestLookupMatchesProviderAPIHost(t *testing.T) {
	catalog := modelsdev.Catalog{
		"custom": {
			API: "https://api.example.test/v1",
			Models: map[string]modelsdev.Model{
				"model-a": {ID: "model-a"},
			},
		},
	}
	if _, ok := catalog.Lookup("my provider", "https://api.example.test/v1/chat", "model-a"); !ok {
		t.Fatal("expected matching API hosts to match")
	}
}

func TestLookupDoesNotConflateProviderPathsOnOneHost(t *testing.T) {
	catalog := modelsdev.Catalog{
		"first": {
			API:    "https://api.example.test/first/v1",
			Models: map[string]modelsdev.Model{"model-a": {ID: "model-a"}},
		},
		"second": {
			API:    "https://api.example.test/second/v1",
			Models: map[string]modelsdev.Model{"model-b": {ID: "model-b"}},
		},
	}
	if _, ok := catalog.Lookup("my provider", "https://api.example.test/second/v1", "model-a"); ok {
		t.Fatal("must not match a provider only because it shares an API host")
	}
}

func TestLookupCursorSubscriptionPreservesAllInputModes(t *testing.T) {
	model := modelsdev.Model{ID: "claude-test"}
	model.Modalities.Input = []string{"text", "image", "pdf"}
	catalog := modelsdev.Catalog{"anthropic": {Models: map[string]modelsdev.Model{"claude-test": model}}}
	result, ok := catalog.Lookup("Cursor Sub", "", "claude-test")
	if !ok || len(result.Modalities.Input) != 3 {
		t.Fatalf("expected all input modalities, got %+v, found %v", result.Modalities.Input, ok)
	}
}

func TestLookupCursorPrefixedGrokPreservesInputModes(t *testing.T) {
	model := modelsdev.Model{ID: "grok-4.7"}
	model.Modalities.Input = []string{"text", "image"}
	catalog := modelsdev.Catalog{"xai": {Models: map[string]modelsdev.Model{"grok-4.7": model}}}
	for _, provider := range []string{"Cursor Sub", "Cursor API", "Custom Cursor"} {
		result, ok := catalog.Lookup(provider, "", "cursor-grok-4.7")
		if !ok || result.ID != model.ID || len(result.Modalities.Input) != 2 {
			t.Fatalf("%s: expected canonical Grok metadata, got %+v, found %v", provider, result, ok)
		}
	}
}

func TestLookupCursorGLMAndMuseOfficialInputs(t *testing.T) {
	for _, entry := range []struct{ provider, id string }{{"zai", "glm-5.3"}, {"meta", "muse-spark-1.3"}} {
		model := modelsdev.Model{ID: entry.id}
		model.Modalities.Input = []string{"text", "image"}
		catalog := modelsdev.Catalog{entry.provider: {Models: map[string]modelsdev.Model{entry.id: model}}}
		result, ok := catalog.Lookup("Cursor Sub", "", "cursor-"+entry.id)
		if !ok || len(result.Modalities.Input) != 2 {
			t.Fatalf("%s: missing official input modes", entry.id)
		}
	}
}

func TestLookupCursorGLMVersionAlias(t *testing.T) {
	model := modelsdev.Model{ID: "glm-5.3-flash"}
	model.Modalities.Input = []string{"text", "image", "video", "pdf"}
	catalog := modelsdev.Catalog{"zai": {Models: map[string]modelsdev.Model{model.ID: model}}}
	result, ok := catalog.Lookup("Cursor Sub", "", "glm-5p3-flash")
	if !ok || len(result.Modalities.Input) != 4 {
		t.Fatal("expected canonical GLM input modalities")
	}
}
