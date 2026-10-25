package test

import (
	"bytes"
	"encoding/json"
	. "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex/chat"
	"io"
	"strings"
	"testing"
)

func TestStreamPropagatesProviderFailures(t *testing.T) {
	for _, tc := range []struct{ event, detail string }{
		{`{"type":"error","error":{"message":"invalid tool history"}}`, "invalid tool history"},
		{`{"type":"response.failed","response":{"error":{"message":"model unavailable"}}}`, "model unavailable"},
		{`{"type":"response.incomplete","response":{"incomplete_details":{"reason":"max_output_tokens"}}}`, "max_output_tokens"},
	} {
		var out bytes.Buffer
		err := RewriteCodexSSEStream(strings.NewReader("data: "+tc.event+"\n\n"), &out, "test-model")
		if err == nil || !strings.Contains(err.Error(), tc.detail) {
			t.Fatalf("error=%v want %s", err, tc.detail)
		}
		if strings.Contains(out.String(), "[DONE]") {
			t.Fatalf("failure marked successful: %s", out.String())
		}
	}
}

func TestCodexStreamRejectsEOFBeforeResponseCompleted(t *testing.T) {
	input := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\"}}\n\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n"
	var out bytes.Buffer
	err := RewriteCodexSSEStream(strings.NewReader(input), &out, "test-model")
	if err != io.ErrUnexpectedEOF {
		t.Fatalf("error=%v, want unexpected EOF", err)
	}
	if strings.Contains(out.String(), "[DONE]") {
		t.Fatalf("incomplete provider response was marked successful: %s", out.String())
	}
}

func TestBufferChatCompletionPreservesCodexResponseFields(t *testing.T) {
	input := strings.Join([]string{
		`data: {"type":"response.created","sequence_number":1,"response":{"id":"resp_1"}}`,
		`data: {"type":"response.output_item.done","sequence_number":2,"item":{"type":"reasoning","encrypted_content":"enc-1"}}`,
		`data: {"type":"response.output_item.added","sequence_number":3,"item":{"type":"function_call","id":"fc-1","call_id":"call-1","name":"shell"}}`,
		`data: {"type":"response.function_call_arguments.delta","sequence_number":4,"item_id":"fc-1","delta":"{\"command\":\"pwd\"}"}`,
		`data: {"type":"response.output_text.delta","sequence_number":5,"delta":"done"}`,
		`data: {"type":"response.completed","sequence_number":6,"response":{"usage":{"input_tokens":8,"output_tokens":3,"total_tokens":11},"output":[{"type":"reasoning","id":"rs_1","encrypted_content":"enc-1","summary":[]}]}}`,
		`data: [DONE]`,
		"",
	}, "\n\n")
	body, err := BufferChatCompletionFromCodexSSE(strings.NewReader(input), "test-model")
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Usage   map[string]any `json:"usage"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content            string `json:"content"`
				ReasoningEncrypted string `json:"reasoning_encrypted_content"`
				ReasoningItem      string `json:"reasoning_item"`
				ToolCalls          []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Choices) != 1 {
		t.Fatalf("choices: %s", body)
	}
	choice := response.Choices[0]
	if choice.Message.Content != "done" || choice.Message.ReasoningEncrypted != "enc-1" || !strings.Contains(choice.Message.ReasoningItem, `"id":"rs_1"`) {
		t.Fatalf("message: %s", body)
	}
	if choice.FinishReason != "tool_calls" || len(choice.Message.ToolCalls) != 1 {
		t.Fatalf("tool calls: %s", body)
	}
	call := choice.Message.ToolCalls[0]
	if call.ID != "call-1" || call.Function.Name != "shell" || call.Function.Arguments != `{"command":"pwd"}` {
		t.Fatalf("tool call: %#v", call)
	}
	if response.Usage["prompt_tokens"] != float64(8) || response.Usage["completion_tokens"] != float64(3) || response.Usage["total_tokens"] != float64(11) {
		t.Fatalf("usage: %#v", response.Usage)
	}
}
