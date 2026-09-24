package test

import (
	"encoding/binary"
	"testing"

	cursorauth "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/cursor"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestDecodeAgentWireEvents(t *testing.T) {
	toolValue, err := proto.Marshal(structpb.NewStringValue("from model"))
	if err != nil {
		t.Fatal(err)
	}
	toolArg := append(pbString(1, "intent"), pbBytes(2, toolValue)...)
	toolArgs := append(pbBytes(2, toolArg), pbString(3, "call-1")...)
	toolArgs = append(toolArgs, pbString(5, "orchestrate")...)

	setBlob := append(pbBytes(1, []byte("blob-id")), pbBytes(2, []byte("history"))...)
	kv := append(pbVarint(1, 12), pbBytes(3, setBlob)...)

	ended := append(pbVarint(1, 10), pbVarint(2, 4)...)
	ended = append(ended, pbVarint(3, 3)...)
	ended = append(ended, pbVarint(4, 2)...)
	ended = append(ended, pbVarint(5, 1)...)

	toolExec := append(pbVarint(1, 8), pbBytes(11, toolArgs)...)
	toolExec = append(toolExec, pbString(15, "exec-8")...)
	contextExec := append(pbVarint(1, 9), pbBytes(10, nil)...)
	contextExec = append(contextExec, pbString(15, "exec-9")...)
	stateExec := append(pbVarint(1, 10), pbBytes(36, nil)...)
	stateExec = append(stateExec, pbString(15, "exec-10")...)
	precheckExec := append(pbVarint(1, 11), pbBytes(42, nil)...)
	precheckExec = append(precheckExec, pbString(15, "exec-11")...)

	tests := []struct {
		name  string
		wire  []byte
		check func(*testing.T, cursorauth.AgentEvent)
	}{
		{
			name: "text delta",
			wire: pbBytes(1, pbBytes(1, pbString(1, "hello"))),
			check: func(t *testing.T, got cursorauth.AgentEvent) {
				if got.Text != "hello" {
					t.Fatalf("text=%q", got.Text)
				}
			},
		},
		{
			name: "thinking delta",
			wire: pbBytes(1, pbBytes(4, pbString(1, "reasoning"))),
			check: func(t *testing.T, got cursorauth.AgentEvent) {
				if got.Thinking != "reasoning" {
					t.Fatalf("thinking=%q", got.Thinking)
				}
			},
		},
		{
			name: "turn usage",
			wire: pbBytes(1, pbBytes(14, ended)),
			check: func(t *testing.T, got cursorauth.AgentEvent) {
				if !got.Ended || got.Usage.PromptTokens != 10 || got.Usage.CompletionTokens != 4 || got.Usage.CachedPromptTokens != 3 || got.Usage.CacheWriteTokens != 2 || got.Usage.ReasoningTokens != 1 {
					t.Fatalf("event=%+v", got)
				}
			},
		},
		{
			name: "tool call and JSON arguments",
			wire: pbBytes(2, toolExec),
			check: func(t *testing.T, got cursorauth.AgentEvent) {
				if got.Tool == nil || got.Tool.ID != "call-1" || got.Tool.Name != "orchestrate" || got.Tool.ExecID != "exec-8" || got.Tool.ExecNumber != 8 || got.Tool.Arguments != `{"intent":"from model"}` {
					t.Fatalf("tool=%+v", got.Tool)
				}
			},
		},
		{
			name: "context request",
			wire: pbBytes(2, contextExec),
			check: func(t *testing.T, got cursorauth.AgentEvent) {
				if got.Context == nil || got.Context.ID != 9 || got.Context.ExecID != "exec-9" {
					t.Fatalf("context=%+v", got.Context)
				}
			},
		},
		{
			name: "MCP state request",
			wire: pbBytes(2, stateExec),
			check: func(t *testing.T, got cursorauth.AgentEvent) {
				if got.MCPState == nil || got.MCPState.ID != 10 || got.MCPState.ExecID != "exec-10" {
					t.Fatalf("MCP state=%+v", got.MCPState)
				}
			},
		},
		{
			name: "MCP allowlist precheck",
			wire: pbBytes(2, precheckExec),
			check: func(t *testing.T, got cursorauth.AgentEvent) {
				if got.Native == nil || got.Native.Kind != "precheck_mcp" {
					t.Fatalf("native=%+v", got.Native)
				}
			},
		},
		{
			name: "interaction query",
			wire: pbBytes(7, append(pbVarint(1, 17), pbBytes(9, nil)...)),
			check: func(t *testing.T, got cursorauth.AgentEvent) {
				if got.Query == nil || got.Query.ID != 17 || got.Query.Kind != 9 {
					t.Fatalf("query=%+v", got.Query)
				}
			},
		},
		{
			name: "KV set blob",
			wire: pbBytes(4, kv),
			check: func(t *testing.T, got cursorauth.AgentEvent) {
				if got.KV == nil || !got.KV.Set || got.KV.ID != 12 || string(got.KV.BlobID) != "blob-id" || string(got.KV.BlobData) != "history" {
					t.Fatalf("KV=%+v", got.KV)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cursorauth.DecodeAgentServerMessage(tc.wire)
			if got.Err != nil {
				t.Fatal(got.Err)
			}
			tc.check(t, got)
		})
	}
}

func TestDecodeAgentWireRejectsMalformedPayload(t *testing.T) {
	got := cursorauth.DecodeAgentServerMessage([]byte{0x0a, 0x02, 0x01})
	if got.Err == nil {
		t.Fatalf("expected malformed payload error, got %+v", got)
	}
}

func TestAssistantTextFromMessage(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
		ok   bool
	}{
		{
			name: "text parts skip reasoning",
			data: `{"role":"assistant","content":[{"type":"reasoning","text":"private"},{"type":"text","text":"Hello"},{"type":"text","text":" world"}]}`,
			want: "Hello world",
			ok:   true,
		},
		{
			name: "string content",
			data: `{"role":"assistant","content":"Hello"}`,
			want: "Hello",
			ok:   true,
		},
		{
			name: "no text part",
			data: `{"role":"assistant","content":[{"type":"reasoning","text":"private"}]}`,
		},
		{
			name: "non assistant role",
			data: `{"role":"user","content":[{"type":"text","text":"Hello"}]}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := cursorauth.AssistantTextFromMessage([]byte(tc.data))
			if got != tc.want || ok != tc.ok {
				t.Fatalf("text=%q ok=%v, want %q ok=%v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func pbString(field int, value string) []byte { return pbBytes(field, []byte(value)) }

func pbBytes(field int, value []byte) []byte {
	out := binary.AppendUvarint(nil, uint64(field<<3|2))
	out = binary.AppendUvarint(out, uint64(len(value)))
	return append(out, value...)
}

func pbVarint(field int, value uint64) []byte {
	out := binary.AppendUvarint(nil, uint64(field<<3))
	return binary.AppendUvarint(out, value)
}
