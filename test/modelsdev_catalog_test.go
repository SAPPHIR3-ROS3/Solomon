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
