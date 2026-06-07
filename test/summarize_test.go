package test

import (
	"bytes"
	"strings"
	"testing"
	"time"

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
	if body != "plain text summary" {
		t.Error("RenderCompactSummaryBody must not mutate its input")
	}
}

func TestRenderCompactSummaryBodyNoMultilinePadding(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{ForceColor: true})
	multi := "short\n" + strings.Repeat("x", 120) + "\nend"
	rendered := commands.RenderCompactSummaryBody(multi)
	plain := termcolor.Plain(rendered)
	for i, line := range strings.Split(plain, "\n") {
		if len(line) != len(strings.TrimRight(line, " ")) {
			t.Fatalf("line %d has trailing padding: len=%d", i, len(line))
		}
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

func TestApplyCompaction_archivesUncompactedRaw(t *testing.T) {
	sess := &chatstore.Session{
		Messages: []chatstore.Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi", ToolCalls: []chatstore.ToolCall{{ID: "c1", Name: "shell", Arguments: "{}"}}},
		},
		CheckpointLast:         3,
		CheckpointBranchSuffix: "a",
		ForkChildCount:         map[int]int{3: 1},
		MainOrphans: []chatstore.MainOrphanSegment{{
			ForkAtInclusive: 4,
			Messages:        []chatstore.Message{{Role: "tool", Content: "ok", ToolCallID: "c1"}},
		}},
		LastCommitOID: "abc123",
	}
	chatstore.ApplyCompaction(sess, "compact body", time.Unix(1700000000, 0))

	if len(sess.Messages) != 1 || sess.Messages[0].Content != "compact body" {
		t.Fatalf("messages=%+v", sess.Messages)
	}
	if len(sess.UncompactedRaw) != 1 {
		t.Fatalf("uncompactedRaw len=%d", len(sess.UncompactedRaw))
	}
	dump := sess.UncompactedRaw[0]
	if dump.CompactAt.Unix() != 1700000000 {
		t.Fatalf("compact_at=%v", dump.CompactAt)
	}
	if len(dump.Messages) != 2 || dump.Messages[1].ToolCalls[0].ID != "c1" {
		t.Fatalf("archived messages=%+v", dump.Messages)
	}
	if dump.CheckpointLast != 3 || dump.CheckpointBranchSuffix != "a" {
		t.Fatalf("checkpoint fields not archived: %+v", dump)
	}
	if dump.ForkChildCount[3] != 1 || len(dump.MainOrphans) != 1 || dump.LastCommitOID != "abc123" {
		t.Fatalf("branch metadata not archived: %+v", dump)
	}
	if len(sess.MainOrphans) != 0 || sess.CheckpointLast != -1 {
		t.Fatalf("live branch state not cleared: orphans=%+v cp=%d", sess.MainOrphans, sess.CheckpointLast)
	}
}

func TestApplyCompaction_appendsOnRecompact(t *testing.T) {
	sess := &chatstore.Session{
		Messages: []chatstore.Message{{Role: "user", Content: "first"}},
	}
	chatstore.ApplyCompaction(sess, "first compact", time.Unix(1, 0))
	sess.Messages = []chatstore.Message{{Role: "user", Content: "after first compact"}}
	chatstore.ApplyCompaction(sess, "second compact", time.Unix(2, 0))

	if len(sess.UncompactedRaw) != 2 {
		t.Fatalf("uncompactedRaw len=%d want 2", len(sess.UncompactedRaw))
	}
	if sess.UncompactedRaw[0].Messages[0].Content != "first" {
		t.Fatalf("first dump=%+v", sess.UncompactedRaw[0].Messages)
	}
	if sess.UncompactedRaw[1].Messages[0].Content != "after first compact" {
		t.Fatalf("second dump=%+v", sess.UncompactedRaw[1].Messages)
	}
}
