package test

import (
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func TestFormatChatTranscript_OmitsReasoning(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "assistant", Content: "visible", ReasoningText: "secret chain"},
	}
	out := commands.FormatChatTranscript(msgs)
	if strings.Contains(strings.ToLower(out), "reasoning") || strings.Contains(out, "secret chain") {
		t.Fatalf("transcript should omit reasoning, got:\n%s", out)
	}
	if !strings.Contains(out, "visible") {
		t.Fatalf("transcript should include visible content")
	}
}

func TestFormatRetainedMessages_OmitsReasoning(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "assistant", Content: "done", ReasoningText: "hidden"},
	}
	out := commands.FormatRetainedMessages(msgs)
	if strings.Contains(out, "Reasoning:") || strings.Contains(out, "hidden") {
		t.Fatalf("retained should omit reasoning, got:\n%s", out)
	}
	if !strings.Contains(out, "done") {
		t.Fatalf("retained should include visible content")
	}
}
