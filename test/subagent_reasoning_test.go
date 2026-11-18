package test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func TestSubagentReasoningCommand(t *testing.T) {
	cfg := &config.Root{ReasoningEffort: "low"}
	var out bytes.Buffer
	var saved []string
	deps := commands.Deps{Cfg: cfg, Out: &out, SaveCfg: func() error {
		saved = append(saved, cfg.SubagentReasoningEffort)
		return nil
	}}
	for _, tc := range []struct {
		input, output string
	}{
		{"reasoning subagent high", "subagent_reasoning_effort=high"},
		{"reasoning subagent", "subagent_reasoning_effort=high"},
		{"reasoning sub med", "subagent_reasoning_effort=medium"},
		{"reasoning subagent reset", "subagent_reasoning_effort=none"},
	} {
		out.Reset()
		if err := commands.Reasoning(deps, strings.Fields(tc.input)); err != nil {
			t.Fatalf("%s: %v", tc.input, err)
		}
		if !strings.Contains(out.String(), tc.output) {
			t.Errorf("%s: got %q, want %q", tc.input, out.String(), tc.output)
		}
	}
	if cfg.ReasoningEffort != "low" || len(saved) != 3 || strings.Join(saved, ",") != "high,medium," {
		t.Fatalf("main effort = %q, saved subagent efforts = %q", cfg.ReasoningEffort, saved)
	}
}

func TestSubagentReasoningCompletion(t *testing.T) {
	env := replcomplete.ReplCompleteEnv{}
	for _, tc := range []struct{ input, suffix string }{
		{"/reasoning su", "bagent"},
		{"/reasoning subagent hi", "gh"},
		{"/reasoning subagent re", "set"},
		{"/reasoning sub re", "set"},
	} {
		line := []rune(tc.input)
		if got, _ := replcomplete.ReplCompleteDo(env, line, len(line)); len(got) != 1 || string(got[0]) != tc.suffix {
			t.Errorf("completion for %q = %q, want %q", tc.input, got, tc.suffix)
		}
		if got := replcomplete.SlashSuggest(env, tc.input, nil); got != tc.input+tc.suffix {
			t.Errorf("suggestion for %q = %q, want %q", tc.input, got, tc.input+tc.suffix)
		}
	}
}
