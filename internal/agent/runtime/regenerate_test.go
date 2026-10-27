package agentruntime

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func TestTruncateForRegeneration(t *testing.T) {
	s := &chatstore.Session{Messages: []chatstore.Message{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "first reply"},
		{Role: "user", Content: "question [img-1]", APIContent: "expanded question [img-1]", CheckpointSeq: 7},
		{Role: "assistant", Content: "tool call", ToolCalls: []chatstore.ToolCall{{ID: "call"}}},
		{Role: "tool", Content: "result", ToolCallID: "call"},
		{Role: "user", Content: "tool_result(legacy)"},
		{Role: "assistant", Content: "old reply"},
		{Role: "user", Content: "/help", GUIHidden: true},
	}}
	original := s.Messages[2]
	if err := truncateForRegeneration(s); err != nil {
		t.Fatal(err)
	}
	if len(s.Messages) != 3 {
		t.Fatalf("got %d messages", len(s.Messages))
	}
	got := s.Messages[2]
	if got.Content != original.Content || got.APIContent != original.APIContent || got.CheckpointSeq != 7 {
		t.Fatalf("user message changed: %+v", got)
	}
	if err := truncateForRegeneration(s); err != nil || len(s.Messages) != 3 {
		t.Fatal("retry must retain the same question", err)
	}
}

func TestTruncateForRegenerationWithoutUser(t *testing.T) {
	for _, s := range []*chatstore.Session{nil, {}, {Messages: []chatstore.Message{{Role: "assistant", Content: "summary"}}}} {
		if err := truncateForRegeneration(s); err == nil {
			t.Fatal("expected missing user error")
		}
	}
}
