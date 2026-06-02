package test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func TestCompactSummaryBodyPlainText(t *testing.T) {
	sep := "================================================================================"
	summary := "Decision: persisted summaries must be plain text."
	retained := "User:\nHello\n\nAssistant:\nHi\n\n"

	body := commands.CompactSummaryBody(sep, summary, retained)

	if !strings.Contains(body, "[Conversation summary]") {
		t.Error("body missing [Conversation summary]")
	}
	if !strings.Contains(body, summary) {
		t.Error("body missing summary text")
	}
	if !strings.Contains(body, "[Retained messages]") {
		t.Error("body missing [Retained messages]")
	}
	if !strings.Contains(body, "User:\nHello") || !strings.Contains(body, "Assistant:\nHi") {
		t.Error("body missing retained block")
	}
	if strings.Contains(body, "\x1b[") {
		t.Error("body must not contain ANSI escape sequences")
	}
}

func TestCompactSummaryBodyOmitsEmptyRetained(t *testing.T) {
	body := commands.CompactSummaryBody("---", "summary only", "  \n")
	if strings.Contains(body, "[Retained messages]") {
		t.Fatalf("empty retained should omit section, got %q", body)
	}
	if strings.Contains(body, "\n\n\n") {
		t.Fatalf("triple blank lines: %q", body)
	}
}

func TestCompactSummaryBodyStructure(t *testing.T) {
	sep := "---"
	summary := "test summary"
	retained := "User:\ntest\n\n"

	body := commands.CompactSummaryBody(sep, summary, retained)
	lines := strings.Split(body, "\n")

	if len(lines) < 12 {
		t.Errorf("expected at least 12 lines, got %d", len(lines))
	}
	if lines[0] != sep {
		t.Errorf("first line should be separator, got %q", lines[0])
	}
	if lines[1] != "[Conversation summary]" {
		t.Errorf("second line should be [Conversation summary], got %q", lines[1])
	}
	if lines[2] != sep {
		t.Errorf("third line should be separator, got %q", lines[2])
	}
}

func TestSummarizeProgressLine(t *testing.T) {
	tests := []struct {
		dots     int
		expected string
	}{
		{0, "Summarizing"},
		{1, "Summarizing."},
		{2, "Summarizing.."},
		{3, "Summarizing..."},
		{5, "Summarizing....."},
	}

	for _, tc := range tests {
		result := commands.SummarizeProgressLine(tc.dots)
		if result != tc.expected {
			t.Errorf("SummarizeProgressLine(%d) = %q, want %q", tc.dots, result, tc.expected)
		}
	}
}

func TestWriteLabeledTranscript_compactSummaryNoModelPrefix(t *testing.T) {
	sep := "================================================================================"
	body := commands.CompactSummaryBody(sep, "summary text", "")
	termcolor.Init(termcolor.InitOptions{Out: &bytes.Buffer{}, NoColor: true})
	var buf bytes.Buffer
	commands.WriteLabeledTranscript(&buf, []chatstore.Message{{Role: "assistant", Content: body}}, "qwen-test", false)
	out := buf.String()
	if strings.Contains(out, "qwen-test:") {
		t.Fatalf("compact summary must not use assistant model prefix, got %q", out)
	}
	if !strings.Contains(out, "[Conversation summary]") {
		t.Fatalf("missing summary header: %q", out)
	}
}

func TestRenderCompactSummaryBodyDoesNotMutatePlain(t *testing.T) {
	body := "plain text summary"
	rendered := commands.RenderCompactSummaryBody(body)

	if strings.Contains(rendered, body) == false {
		t.Error("rendered output should contain the original plain text")
	}
	// Verify that rendering doesn't mutate the input
	if body != "plain text summary" {
		t.Error("RenderCompactSummaryBody must not mutate its input")
	}
}

func TestSummarizeProgressStopWaitsForGoroutine(t *testing.T) {
	var buf strings.Builder
	p := commands.NewSummarizeProgress(&buf)

	p.Stop()

	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("progress output should end with newline after Stop(), got: %q", out)
	}
	if !strings.Contains(out, "Summarizing") {
		t.Errorf("output missing 'Summarizing', got: %q", out)
	}
}
