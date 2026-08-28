package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func TestParseReasoningEffortToken(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "none", want: "none"},
		{input: "med", want: "medium"},
		{input: "medium", want: "medium"},
		{input: "high", want: "high"},
		{input: "xhigh", want: "xhigh"},
		{input: "extra-high", want: "xhigh"},
		{input: "extra_high", want: "xhigh"},
		{input: "extra high", want: "xhigh"},
		{input: "MAX", want: "max"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := config.ParseReasoningEffortToken(tt.input)
			if err != nil {
				t.Fatalf("ParseReasoningEffortToken(%q) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ParseReasoningEffortToken(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGlobalReasoningEffortPreservesOpenAIExtendedLevels(t *testing.T) {
	for _, effort := range []string{"xhigh", "max"} {
		cfg := &config.Root{ReasoningEffort: effort}
		if got := string(cfg.GlobalReasoningEffort()); got != effort {
			t.Fatalf("GlobalReasoningEffort() for %q = %q, want %q", effort, got, effort)
		}
	}
}
