package chatstore

import "testing"

func TestRepairInterruptedToolCalls(t *testing.T) {
	s := &Session{Messages: []Message{
		{Role: "assistant", ToolCalls: []ToolCall{{ID: "done"}, {ID: "lost"}}},
		{Role: "tool", ToolCallID: "done", Content: "actual result"},
		{Role: "user", Content: "retry"},
		{Role: "assistant", ToolCalls: []ToolCall{{ID: "also-lost"}}},
	}}
	if n := RepairInterruptedToolCalls(s); n != 2 {
		t.Fatalf("repaired %d", n)
	}
	if len(s.Messages) != 6 || s.Messages[1].Content != "actual result" || s.Messages[2].ToolCallID != "lost" || s.Messages[3].Role != "user" || s.Messages[5].ToolCallID != "also-lost" {
		t.Fatalf("history: %+v", s.Messages)
	}
	if n := RepairInterruptedToolCalls(s); n != 0 {
		t.Fatalf("not idempotent: %d", n)
	}
}
