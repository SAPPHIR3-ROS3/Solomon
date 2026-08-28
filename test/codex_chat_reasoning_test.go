package test

import (
	"encoding/json"
	"testing"

	codexchat "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex/chat"
)

func TestChatCompletionToCodexBodyPreservesExtendedOpenAILevels(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "xhigh", want: "xhigh"},
		{input: "extra high", want: "xhigh"},
		{input: "max", want: "max"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			body, err := codexchat.ChatCompletionToCodexBody(map[string]any{"reasoning_effort": tt.input})
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(body, &decoded); err != nil {
				t.Fatal(err)
			}
			reasoning, ok := decoded["reasoning"].(map[string]any)
			if !ok {
				t.Fatalf("reasoning = %#v", decoded["reasoning"])
			}
			if got := reasoning["effort"]; got != tt.want {
				t.Fatalf("effort = %v, want %q", got, tt.want)
			}
		})
	}
}
