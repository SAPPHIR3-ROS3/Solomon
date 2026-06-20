package test

import (
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt"
)

func TestRenderBtwSystemOmitsToolDump(t *testing.T) {
	t.Parallel()
	sys, err := prompt.RenderBtwSystem(prompt.Data{
		Language:              "Italian",
		UserName:              "Oni",
		WorkspaceAbsolutePath: "/tmp/ws",
		Tools:                 "SHOULD_NOT_APPEAR",
		Syntax:                "SHOULD_NOT_APPEAR",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sys, "SHOULD_NOT_APPEAR") {
		t.Fatalf("tool fields leaked into btw system: %s", sys)
	}
	if !strings.Contains(sys, "Italian") {
		t.Fatal("expected language in btw system prompt")
	}
	if !strings.Contains(sys, "Side question") {
		t.Fatal("expected btw section in system prompt")
	}
}

func TestRenderBtwUserIncludesTranscriptAndQuestion(t *testing.T) {
	t.Parallel()
	user, err := prompt.RenderBtw(prompt.BtwData{
		Transcript: "User:\nhi\n\n",
		Question:   "what?",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(user, "hi") || !strings.Contains(user, "what?") {
		t.Fatalf("unexpected user prompt: %s", user)
	}
}
