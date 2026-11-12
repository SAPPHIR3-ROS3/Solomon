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

func TestFormatChatTranscript_previousSummaryOnce(t *testing.T) {
	separator := "---"
	body := separator + "\n[Conversation summary]\n" + separator + "\n\nFirst summary.\n\n" + separator + "\n[Conversation summary]\n" + separator + "\n\nSecond summary.\n\n" + separator + "\n[Retained messages]\n" + separator + "\n\nUser:\nOld question\n"
	out := commands.FormatChatTranscript([]chatstore.Message{{Role: "assistant", Content: body}})
	if strings.Contains(out, "[Conversation summary]") || strings.Contains(out, "[Retained messages]") || strings.Contains(out, "First summary.") || strings.Contains(out, "Old question") {
		t.Fatalf("transcript pasted the summary envelope: %q", out)
	}
	if !strings.Contains(out, "Second summary.") {
		t.Fatalf("transcript = %q", out)
	}
}

func TestFormatRetainedMessages_skipsPreviousSummary(t *testing.T) {
	separator := "---"
	body := separator + "\n[Conversation summary]\n" + separator + "\n\nKeep this out of the next summary.\n" + separator
	out := commands.FormatRetainedMessages([]chatstore.Message{
		{Role: "assistant", Content: body},
		{Role: "user", Content: "Current question"},
	})
	if strings.Contains(out, "Keep this out") || strings.Contains(out, "[Conversation summary]") {
		t.Fatalf("retained pasted the previous summary: %q", out)
	}
	if !strings.Contains(out, "Current question") {
		t.Fatalf("retained = %q", out)
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
