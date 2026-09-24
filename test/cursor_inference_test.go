package test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cursorauth "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/cursor"
)

const cursorParameterizedCatalog = `{"models":[{"name":"grok-4.6","variants":[{"legacySlug":"cursor-grok-4.6-xhigh-fast","parameterValues":[{"id":"effort","value":"xhigh"},{"id":"fast","value":"true"}]}]},{"name":"composer-2.5","variants":[{"legacySlug":"composer-2.5","isDefaultNonMaxConfig":true,"parameterValues":[{"id":"fast","value":"false"}]}]}]}`

func cursorInferenceFixture(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	catalog := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, cursorParameterizedCatalog)
	}))
	t.Cleanup(catalog.Close)
	t.Cleanup(cursorauth.SetModelsEndpointForTest(catalog.URL))
	chat := httptest.NewServer(handler)
	t.Cleanup(chat.Close)
	t.Cleanup(cursorauth.SetChatEndpointForTest(chat.URL))
}

// Inspect the actual wire request independently of the production encoder.
func cursorWireFields(t *testing.T, data []byte, number uint64) [][]byte {
	t.Helper()
	var values [][]byte
	for len(data) > 0 {
		tag, n := binary.Uvarint(data)
		if n <= 0 {
			t.Fatal("invalid tag")
		}
		data = data[n:]
		var value []byte
		switch tag & 7 {
		case 0:
			_, n = binary.Uvarint(data)
			if n <= 0 {
				t.Fatal("invalid varint")
			}
			value, data = data[:n], data[n:]
		case 2:
			size, n := binary.Uvarint(data)
			if n <= 0 || size > uint64(len(data)-n) {
				t.Fatal("invalid field length")
			}
			data = data[n:]
			value, data = data[:size], data[size:]
		default:
			t.Fatalf("unexpected wire type %d", tag&7)
		}
		if tag>>3 == number {
			values = append(values, value)
		}
	}
	return values
}

func cursorWireField(t *testing.T, data []byte, number uint64) []byte {
	t.Helper()
	values := cursorWireFields(t, data, number)
	if len(values) != 1 {
		t.Fatalf("field %d: got %d values", number, len(values))
	}
	return values[0]
}

func cursorTestBytes(number uint64, value []byte) []byte {
	b := binary.AppendUvarint(nil, number<<3|2)
	b = binary.AppendUvarint(b, uint64(len(value)))
	return append(b, value...)
}

func cursorTestVarint(number, value uint64) []byte {
	b := binary.AppendUvarint(nil, number<<3)
	return binary.AppendUvarint(b, value)
}

func cursorTestTextFrame(number uint64, text string) []byte {
	return cursorInferenceTestFrame(cursorTestBytes(number, cursorTestBytes(1, []byte(text))), 0)
}

func TestInferenceRequestUsesStructuredModel(t *testing.T) {
	cursorInferenceFixture(t, func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		if len(raw) < 5 {
			t.Error("missing Connect frame")
			return
		}
		request := raw[5:]
		selection := cursorWireField(t, request, 7)
		if got := string(cursorWireField(t, selection, 1)); got != "grok-4.6" {
			t.Errorf("model=%q", got)
		}
		parameters := map[string]string{}
		for _, p := range cursorWireFields(t, selection, 3) {
			parameters[string(cursorWireField(t, p, 1))] = string(cursorWireField(t, p, 2))
		}
		if parameters["effort"] != "xhigh" || parameters["fast"] != "true" {
			t.Errorf("parameters=%v", parameters)
		}
		messages := cursorWireFields(t, request, 1)
		if len(messages) != 2 {
			t.Errorf("messages=%d", len(messages))
			return
		}
		if got := string(cursorWireField(t, messages[1], 2)); got != " hi " {
			t.Errorf("content=%q", got)
		}
		role, _ := binary.Uvarint(cursorWireField(t, messages[0], 1))
		if role != 4 {
			t.Errorf("system role=%d", role)
		}
		_, _ = w.Write(cursorTestTextFrame(1, "OK"))
		_, _ = w.Write(cursorInferenceTestFrame([]byte(`{}`), 2))
	})
	_, err := cursorauth.StreamChat(context.Background(), "test", "cursor-grok-4.6-xhigh-fast", []cursorauth.ChatTurn{{Role: "system", Content: "system"}, {Role: "user", Content: " hi "}}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
}

func TestInferenceRequestIncludesToolsAndToolHistory(t *testing.T) {
	cursorInferenceFixture(t, func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		request := raw[5:]
		definitions := cursorWireFields(t, request, 2)
		if len(definitions) != 1 {
			t.Fatalf("tool definitions=%d", len(definitions))
		}
		definition := definitions[0]
		if got := string(cursorWireField(t, definition, 1)); got != "write_note" {
			t.Fatalf("tool name=%q", got)
		}
		if got := string(cursorWireField(t, definition, 2)); got != "Write a note" {
			t.Fatalf("tool description=%q", got)
		}
		if len(cursorWireFields(t, definition, 3)) != 1 {
			t.Fatal("tool parameters Struct missing")
		}
		messages := cursorWireFields(t, request, 1)
		if len(messages) != 3 {
			t.Fatalf("history messages=%d", len(messages))
		}
		assistantCalls := cursorWireFields(t, messages[0], 4)
		if len(assistantCalls) != 1 || string(cursorWireField(t, assistantCalls[0], 2)) != "write_note" {
			t.Fatalf("assistant tool calls=%v", assistantCalls)
		}
		if len(cursorWireFields(t, messages[1], 6)) != 1 {
			t.Fatal("tool result content missing")
		}
		_, _ = w.Write(cursorTestTextFrame(1, "done"))
		_, _ = w.Write(cursorInferenceTestFrame([]byte(`{}`), 2))
	})
	result, err := cursorauth.StreamChatWithTools(context.Background(), "test", "composer-2.5", []cursorauth.ChatTurn{
		{Role: "assistant", ToolCalls: []cursorauth.ChatToolCall{{ID: "call-1", Name: "write_note", Arguments: `{"intent":"save","path":"note.txt"}`}}},
		{Role: "tool", ToolCallID: "call-1", ToolName: "write_note", Content: `{"ok":true}`},
		{Role: "user", Content: "Continue."},
	}, []cursorauth.ChatTool{{Name: "write_note", Description: "Write a note", Parameters: map[string]any{
		"type":       "object",
		"properties": map[string]any{"path": map[string]any{"type": "string"}},
		"required":   []any{"path"},
	}}}, io.Discard)
	if err != nil || result.Content != "done" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestInferenceToolCallResponse(t *testing.T) {
	toolPart := append(cursorTestBytes(1, []byte("call-7")), cursorTestBytes(2, []byte("write_note"))...)
	toolPart = append(toolPart, cursorTestBytes(3, []byte(`{"intent":"save","path":"note.txt"}`))...)
	toolPart = append(toolPart, cursorTestVarint(4, 1)...)
	stream := cursorInferenceTestFrame(cursorTestBytes(2, toolPart), 0)
	stream = append(stream, cursorInferenceTestFrame([]byte(`{}`), 2)...)
	cursorInferenceFixture(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(stream) })
	result, err := cursorauth.StreamChatWithTools(context.Background(), "test", "composer-2.5", nil, []cursorauth.ChatTool{{Name: "write_note"}}, io.Discard)
	if err != nil || result.FinishReason != "tool_calls" || len(result.ToolCalls) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	call := result.ToolCalls[0]
	if call.ID != "call-7" || call.Name != "write_note" || call.Arguments != `{"intent":"save","path":"note.txt"}` {
		t.Fatalf("tool call=%+v", call)
	}
}

func TestChatUpdateRequiredDetail(t *testing.T) {
	payload := []byte(`{"error":{"code":"resource_exhausted","message":"Error","details":[{"type":"aiserver.v1.ErrorDetails","debug":{"error":"ERROR_GPT_4_VISION_PREVIEW_RATE_LIMIT","details":{"title":"Update Required","detail":"Your version of Cursor is no longer supported. Please update to the latest version at cursor.com/downloads to continue."}}}]}}`)
	for _, framed := range []bool{false, true} {
		t.Run(map[bool]string{false: "HTTP", true: "Connect"}[framed], func(t *testing.T) {
			cursorInferenceFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if framed {
					_, _ = w.Write(cursorInferenceTestFrame(payload, 2))
				} else {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write(payload)
				}
			})
			_, err := cursorauth.StreamChat(context.Background(), "test", "composer-2.5", nil, io.Discard)
			if err == nil {
				t.Fatal("error missing")
			}
			message := err.Error()
			if !strings.Contains(message, "Update Required: ") || !strings.HasSuffix(message, "to continue.") || strings.Contains(message, "resource_exhausted") || strings.ContainsRune(message, '\x00') {
				t.Fatalf("error=%q", message)
			}
		})
	}
}

func TestInferenceStreamFrames(t *testing.T) {
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	_, _ = zw.Write(cursorTestBytes(1, cursorTestBytes(1, []byte(" hello"))))
	_ = zw.Close()
	stream := cursorInferenceTestFrame(compressed.Bytes(), 1)
	stream = append(stream, cursorTestTextFrame(9, "private reasoning")...)
	stream = append(stream, cursorTestTextFrame(4, "metadata-id")...)
	stream = append(stream, cursorTestTextFrame(1, " world\n")...)
	stream = append(stream, cursorInferenceTestFrame([]byte(`{}`), 2)...)
	cursorInferenceFixture(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(stream) })
	var output strings.Builder
	got, err := cursorauth.StreamChat(context.Background(), "test", "composer-2.5", nil, &output)
	if err != nil || got != " hello world\n" || output.String() != got {
		t.Fatalf("text=%q output=%q err=%v", got, output.String(), err)
	}
	stream = stream[:len(stream)-2]
	if _, err = cursorauth.StreamChat(context.Background(), "test", "composer-2.5", nil, io.Discard); err == nil {
		t.Fatal("truncated trailer accepted")
	}
}

func TestInferenceErrorAfterText(t *testing.T) {
	cursorInferenceFixture(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(cursorTestTextFrame(1, "partial"))
		_, _ = w.Write(cursorTestTextFrame(8, "provider failed"))
	})
	var output strings.Builder
	_, err := cursorauth.StreamChat(context.Background(), "test", "composer-2.5", nil, &output)
	if output.String() != "partial" || err == nil || !strings.Contains(err.Error(), "provider failed") {
		t.Fatalf("output=%q err=%v", output.String(), err)
	}
}

func TestInferenceCatalogMapping(t *testing.T) {
	calls := 0
	cursorInferenceFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write(cursorTestTextFrame(1, "OK"))
		_, _ = w.Write(cursorInferenceTestFrame([]byte(`{}`), 2))
	})
	for _, slug := range []string{"cursor-grok-4.6-xhigh-fast", "grok-4.6-xhigh-fast", "cursor-composer-2.5"} {
		if _, err := cursorauth.StreamChat(context.Background(), "test", slug, nil, io.Discard); err != nil {
			t.Fatalf("slug=%s err=%v", slug, err)
		}
	}
	if _, err := cursorauth.StreamChat(context.Background(), "test", "cursor-grok-4.6-imaginary", nil, io.Discard); err == nil {
		t.Fatal("invented variant accepted")
	}
	if calls != 3 {
		t.Fatalf("chat requests=%d", calls)
	}
}
