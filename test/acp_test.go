package test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/acp"
)

type protocolRunner struct{ turns int }

func (r *protocolRunner) Close() error { return nil }
func (r *protocolRunner) Prompt(ctx context.Context, text string, emit func(acp.Update)) error {
	r.turns++
	if text == "wait" {
		<-ctx.Done()
		return ctx.Err()
	}
	emit(acp.Update{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": text}})
	return nil
}
func TestACPConversationAndCancellation(t *testing.T) {
	input, writer := io.Pipe()
	reader, output := io.Pipe()
	runner := &protocolRunner{}
	server := acp.New(output, func(context.Context, string, []json.RawMessage) (acp.Runner, error) { return runner, nil })
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _ = server.Serve(context.Background(), input); output.Close() }()
	defer func() { writer.Close(); reader.Close(); wg.Wait() }()
	scan := bufio.NewScanner(reader)
	send := func(id int, method string, params any) {
		t.Helper()
		if err := json.NewEncoder(writer).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
			t.Fatal(err)
		}
	}
	receive := func() map[string]any {
		t.Helper()
		if !scan.Scan() {
			t.Fatal("missing response")
		}
		var m map[string]any
		if err := json.Unmarshal(scan.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	send(1, "session/new", map[string]any{"cwd": t.TempDir()})
	if receive()["error"] == nil {
		t.Fatal("accepted session before initialization")
	}
	send(2, "initialize", map[string]any{"protocolVersion": 1})
	if receive()["result"] == nil {
		t.Fatal("initialization failed")
	}
	send(3, "session/new", map[string]any{"cwd": t.TempDir(), "mcpServers": []any{}})
	id := receive()["result"].(map[string]any)["sessionId"]
	for i := 0; i < 2; i++ {
		send(4+i, "session/prompt", map[string]any{"sessionId": id, "prompt": []any{map[string]any{"type": "text", "text": "hello"}}})
		if receive()["method"] != "session/update" {
			t.Fatal("missing streaming notification")
		}
		if receive()["result"].(map[string]any)["stopReason"] != "end_turn" {
			t.Fatal("incorrect stop reason")
		}
	}
	send(6, "session/prompt", map[string]any{"sessionId": id, "prompt": []any{map[string]any{"type": "text", "text": "wait"}}})
	send(7, "session/cancel", map[string]any{"sessionId": id})
	if receive()["result"].(map[string]any)["stopReason"] != "cancelled" {
		t.Fatal("cancellation failed")
	}
	send(8, "unknown", map[string]any{})
	if receive()["error"] == nil {
		t.Fatal("unknown method accepted")
	}
}
