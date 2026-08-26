package chat

import "testing"

func TestBuildReasoningFromChatPreservesExtendedOpenAILevels(t *testing.T) {
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
			got := buildReasoningFromChat(map[string]any{"reasoning_effort": tt.input})
			if got["effort"] != tt.want {
				t.Fatalf("effort = %v, want %q", got["effort"], tt.want)
			}
		})
	}
}
