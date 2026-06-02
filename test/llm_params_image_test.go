package test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
)

func TestNeutralizeLiteralImgPlaceholders(t *testing.T) {
	got := chatstore.NeutralizeLiteralImgPlaceholders("see [img-1] in docs")
	want := "see `[img-1]` in docs"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestScrubLiteralImgPlaceholdersForAPI_reasoning(t *testing.T) {
	got := chatstore.ScrubLiteralImgPlaceholdersForAPI("plan around [img-0] token")
	if strings.Contains(got, "[img-") {
		t.Fatalf("got %q", got)
	}
}

func TestScrubLiteralImgPlaceholdersForAPI_danglingBracket(t *testing.T) {
	got := chatstore.ScrubLiteralImgPlaceholdersForAPI("cerco `[img-` nel codice")
	if strings.Contains(got, "[img-") {
		t.Fatalf("got %q", got)
	}
}

func TestScrubLiteralImgPlaceholdersForAPI_escapedRgPattern(t *testing.T) {
	got := chatstore.ScrubLiteralImgPlaceholdersForAPI(`rg -n \"\[img-\"`)
	if strings.Contains(got, "[img-") {
		t.Fatalf("got %q", got)
	}
}

func TestMessageParamsStripsAssistantToolCallImgLiterals(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "user", Content: "go"},
		{Role: "assistant", ToolCalls: []chatstore.ToolCall{{
			ID: "c1", Name: "editFile", Arguments: `{"newString":"tag [img-0] here"}`,
		}}},
	}
	params := llm.MessageParams("", msgs, nil)
	if len(params) < 3 {
		t.Fatalf("params len %d", len(params))
	}
	ap := params[2].OfAssistant
	if ap == nil || len(ap.ToolCalls) == 0 {
		t.Fatal("expected assistant tool_calls")
	}
	args := ap.ToolCalls[0].OfFunction.Function.Arguments
	if strings.Contains(args, "[img-") {
		t.Fatalf("tool call args not stripped: %q", args)
	}
}

func TestMessageParamsStripsAssistantImgLiterals(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "placeholder [img-0] is documented in TODO"},
	}
	params := llm.MessageParams("", msgs, nil)
	if len(params) < 3 {
		t.Fatalf("params len %d", len(params))
	}
	ap := params[2].OfAssistant
	if ap == nil || ap.Content.OfString.Value == "" {
		t.Fatal("expected assistant string content")
	}
	if strings.Contains(ap.Content.OfString.Value, "[img-") {
		t.Fatalf("assistant content not stripped: %q", ap.Content.OfString.Value)
	}
}

func TestStripFalseImgPlaceholdersFromNonUserSession_toolAndArgs(t *testing.T) {
	s := &chatstore.Session{Messages: []chatstore.Message{
		{Role: "tool", Content: `{"output":"see [img-n] in TODO"}`},
		{Role: "assistant", ToolCalls: []chatstore.ToolCall{{
			ID: "c1", Name: "shell", Arguments: `{"command":"rg '[img-0]'"}`,
		}}},
	}}
	n := chatstore.StripFalseImgPlaceholdersFromNonUserSession(s)
	if n == 0 {
		t.Fatal("expected strip count > 0")
	}
	if strings.Contains(s.Messages[0].Content, "[img-") {
		t.Fatalf("tool content: %q", s.Messages[0].Content)
	}
	if strings.Contains(s.Messages[1].ToolCalls[0].Arguments, "[img-") {
		t.Fatalf("tool call args: %q", s.Messages[1].ToolCalls[0].Arguments)
	}
}

func TestNormalizeSummaryWhitespace(t *testing.T) {
	in := "a\n\n\n\nb\n\n"
	got := chatstore.NormalizeSummaryWhitespace(in)
	if strings.Contains(got, "\n\n\n") {
		t.Fatalf("got %q", got)
	}
}

func TestScrubSummaryImgWorkflowLines(t *testing.T) {
	in := "Decisioni:\n- Tag immagine: saltati nel parsing\n- Percorsi: ok\n"
	got := chatstore.ScrubSummaryImgWorkflowLines(in)
	if strings.Contains(strings.ToLower(got), "tag immagine") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "Percorsi") {
		t.Fatalf("got %q", got)
	}
}

func TestBuildUserContentPartsOmitsStaleImgTag(t *testing.T) {
	parts := llm.BuildUserContentParts("[img-0]", nil)
	if len(parts) != 1 || parts[0].OfText == nil {
		t.Fatalf("want single text part, got %+v", parts)
	}
	if got := parts[0].GetText(); got == nil || strings.TrimSpace(*got) != "" {
		t.Fatalf("unresolved bare tag must not reach API, got %q", *got)
	}
	if strings.Contains(*parts[0].GetText(), "[img-") {
		t.Fatalf("API text still contains img literal: %q", *parts[0].GetText())
	}
	parts = llm.BuildUserContentParts("pre [img-1] suf", map[int]string{1: filepath.Join(t.TempDir(), "nope.png")})
	if len(parts) != 1 || parts[0].OfText == nil {
		t.Fatalf("want flattened text-only, got %+v", parts)
	}
	got := parts[0].GetText()
	if got == nil || *got != "pre  suf" {
		t.Fatalf("got %q", *got)
	}
	if strings.Contains(*got, "[img-") {
		t.Fatalf("stale tag in API text: %q", *got)
	}
}

func TestMessageParamsUserPlainTextNoImgLiteral(t *testing.T) {
	msgs := []chatstore.Message{{Role: "user", Content: "ciao"}}
	params := llm.MessageParams("system ok", msgs, map[int]string{0: "/tmp/orphan.png"})
	if len(params) < 2 {
		t.Fatalf("params len %d", len(params))
	}
	up := params[1].OfUser
	if up == nil {
		t.Fatal("expected user message")
	}
	var text string
	if up.Content.OfString.Valid() {
		text = up.Content.OfString.Value
	}
	if strings.Contains(text, "[img-") {
		t.Fatalf("user API text: %q", text)
	}
	sys := params[0].OfSystem
	if sys == nil || strings.Contains(sys.Content.OfString.Value, "[img-") {
		t.Fatalf("system must not contain img literal: %+v", sys)
	}
}
