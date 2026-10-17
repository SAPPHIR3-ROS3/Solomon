package test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/anthropic/claude"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelsapi"
)

// Opt-in only: opens a real browser and makes one short subscription request.
// It does not read or overwrite the user's saved Solomon/Claude Code credentials.
func TestClaudeBrowserLoginLive(t *testing.T) {
	if os.Getenv("SOLOMON_CLAUDE_LOGIN_LIVE") != "1" {
		t.Skip("set SOLOMON_CLAUDE_LOGIN_LIVE=1 to test with a real Claude subscription")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	tokens, err := claude.Login(ctx, os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := modelsapi.ListAnthropicWithClient(ctx, config.AnthropicPlatformBase, tokens.AccessToken, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	model := ""
	for _, id := range ids {
		if strings.Contains(strings.ToLower(id), "haiku") {
			model = id
			break
		}
	}
	if model == "" {
		for _, id := range ids {
			if strings.Contains(strings.ToLower(id), "sonnet") {
				model = id
				break
			}
		}
	}
	if model == "" {
		t.Fatalf("no Haiku or Sonnet model in live catalog (%d entries)", len(ids))
	}
	backend := llm.NewAnthropicBackend(config.AnthropicPlatformBase, llm.AnthropicAuthFromOAuthBearer(tokens.AccessToken))
	text, _, err := backend.StreamText(ctx, llm.SimpleCompletionRequest{Model: model, User: "Reply exactly: Solomon Anthropic OK", ForceDisableReasoning: true, Cfg: &config.Root{MaxResponseTokens: 64}}, io.Discard, llm.StreamOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(text) == "" {
		t.Fatal("subscription returned no text")
	}
	t.Logf("Browser redirect, token exchange, live catalog and streaming passed with %s", model)
}
