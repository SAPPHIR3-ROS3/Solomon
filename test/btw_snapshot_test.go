package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/btw"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func TestCompleteMessagesTrimsIncompleteAssistantTools(t *testing.T) {
	t.Parallel()
	msgs := []chatstore.Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "ok", ToolCalls: []chatstore.ToolCall{{ID: "t1", Name: "readFile"}}},
	}
	got := btw.CompleteMessages(msgs)
	if len(got) != 1 || got[0].Role != "user" {
		t.Fatalf("got %d messages, want user-only tail trimmed: %+v", len(got), got)
	}
}

func TestCompleteMessagesKeepsResolvedTools(t *testing.T) {
	t.Parallel()
	msgs := []chatstore.Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "ok", ToolCalls: []chatstore.ToolCall{{ID: "t1", Name: "readFile"}}},
		{Role: "tool", ToolCallID: "t1", Content: `{"ok":true}`},
	}
	got := btw.CompleteMessages(msgs)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
}

func TestCompleteMessagesKeepsAssistantWithoutTools(t *testing.T) {
	t.Parallel()
	msgs := []chatstore.Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "done"},
	}
	got := btw.CompleteMessages(msgs)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}
