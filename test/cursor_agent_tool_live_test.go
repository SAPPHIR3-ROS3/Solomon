package test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
)

func TestCursorAgentToolLive(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	model := os.Getenv("SOLOMON_CURSOR_TOOL_MODEL")
	if model == "" {
		model = "cursor-grok-4.6-high"
	}
	msgs := []chatstore.Message{{Role: "user", Content: "Call read_first_line now, then reply with exactly the line returned by that tool. Do not guess the line."}}
	tools := []llm.ToolDef{{Name: "read_first_line", Description: "Reads the first line of go.mod in the current workspace. No arguments.", Parameters: map[string]any{"type": "object", "properties": map[string]any{}}}}
	first, err := backend.StreamTurn(ctx, llm.TurnRequest{Model: model, System: "Use the provided tool to read the requested file.", Messages: msgs, Tools: tools}, io.Discard, llm.StreamOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %d and content %q", len(first.ToolCalls), first.Content)
	}
	call := first.ToolCalls[0]
	if call.Name != "read_first_line" || call.ID == "" {
		t.Fatalf("unexpected tool call: %#v", call)
	}
	t.Logf("tool call: %s %s", call.Name, call.Arguments)
	raw, err := os.ReadFile("../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	line, _, _ := strings.Cut(string(raw), "\n")
	msgs = append(msgs, chatstore.Message{Role: "assistant", Content: first.Content, ToolCalls: []chatstore.ToolCall{{ID: call.ID, Name: call.Name, Arguments: call.Arguments}}})
	msgs = append(msgs, chatstore.Message{Role: "tool", ToolCallID: call.ID, Content: line})
	second, err := backend.StreamTurn(ctx, llm.TurnRequest{Model: model, System: "Use the provided tool to read the requested file.", Messages: msgs, Tools: tools}, io.Discard, llm.StreamOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(second.Content, strings.TrimSpace(line)) {
		t.Fatalf("tool result not used: %q", second.Content)
	}
	msgs = append(msgs, chatstore.Message{Role: "assistant", Content: second.Content})
	msgs = append(msgs, chatstore.Message{Role: "user", Content: "What was the first line of go.mod that the tool returned earlier? Reply with only that line."})
	third, err := backend.StreamTurn(ctx, llm.TurnRequest{Model: model, System: "Use the previous conversation to answer.", Messages: msgs, Tools: tools}, io.Discard, llm.StreamOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(third.Content, strings.TrimSpace(line)) {
		t.Fatalf("history not used: %q", third.Content)
	}
}
