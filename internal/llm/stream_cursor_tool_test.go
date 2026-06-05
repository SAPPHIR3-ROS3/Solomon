package llm

import "testing"

func TestParseCursorToolEventFromChunkRawJSON(t *testing.T) {
	raw := `{"solomon_cursor_tool_event":{"name":"Read","status":"running","args":{"path":"a.go"}}}`
	got := parseCursorToolEventFromChunkRawJSON(raw)
	if got == "" {
		t.Fatal("expected event json")
	}
	if got != `{"name":"Read","status":"running","args":{"path":"a.go"}}` {
		t.Fatalf("unexpected payload: %s", got)
	}
}
