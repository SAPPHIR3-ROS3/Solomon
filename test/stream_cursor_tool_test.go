package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
)

func TestParseCursorToolEventFromChunkRawJSON(t *testing.T) {
	raw := `{"solomon_cursor_tool_event":{"name":"Read","status":"running","args":{"path":"a.go"}}}`
	got := llm.ParseCursorToolEventFromChunkRawJSON(raw)
	if got == "" {
		t.Fatal("expected event json")
	}
	if got != `{"name":"Read","status":"running","args":{"path":"a.go"}}` {
		t.Fatalf("unexpected payload: %s", got)
	}
}
