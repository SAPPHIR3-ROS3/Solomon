package test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	cursorauth "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
)

func TestCursorAgentDirectLive(t *testing.T) {
	if os.Getenv("SOLOMON_CURSOR_DIRECT_LIVE") != "1" {
		t.Skip("live subscription test")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	p := config.ProviderByName(cfg, config.ProviderNameCursorSub)
	if p == nil {
		t.Fatal("Cursor Sub provider missing")
	}
	key := p.APIKey
	if key == "" {
		session, err := config.ResolveCursorSessionBearer(context.Background(), cfg, p)
		if err != nil {
			t.Fatal(err)
		}
		key, err = cursorauth.MintUserAPIKey(context.Background(), session)
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	model := os.Getenv("SOLOMON_CURSOR_DIRECT_MODEL")
	if model == "" {
		model = "cursor-grok-4.6-high"
	}
	s, err := cursorauth.OpenAgentSession(ctx, key, model, "Reply briefly.", "Reply with OK only.", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var response string
	for {
		event, ok := s.Next(ctx)
		if !ok {
			break
		}
		if event.Err != nil {
			t.Fatal(event.Err)
		}
		if event.Context != nil {
			if err := s.ContextResult(event.Context.ID, event.Context.ExecID, "Reply briefly.", nil); err != nil {
				t.Fatal(err)
			}
		}
		if event.KV != nil {
			if err := s.KVResult(*event.KV); err != nil {
				t.Fatal(err)
			}
		}
		if event.Text != "" {
			response += event.Text
		}
		if event.Ended {
			if response == "" {
				response, _ = s.LatestAssistantText()
			}
			break
		}
	}
	if strings.TrimSpace(response) != "OK" {
		t.Fatalf("unexpected response %q", response)
	}
}

func TestCursorAgentModelListLive(t *testing.T) {
	if os.Getenv("SOLOMON_CURSOR_DIRECT_LIVE") != "1" {
		t.Skip("live subscription test")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	p := config.ProviderByName(cfg, config.ProviderNameCursorSub)
	if p == nil {
		t.Fatal("Cursor Sub provider missing")
	}
	backend, err := llm.NewCompletionBackend(context.Background(), cfg, p)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	models, err := backend.ListModels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) == 0 {
		t.Fatal("empty Cursor model list")
	}
}
