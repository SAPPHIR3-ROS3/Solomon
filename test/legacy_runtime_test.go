package test

import (
	"errors"
	"strings"
	"testing"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/prompt"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"
)

func TestResolveTurnInvocations_nativePreferredWhenOptional(t *testing.T) {
	r := &agentruntime.Runtime{
		Mode: "build",
		Cfg:  &config.Root{Tools: config.Tools{Legacy: true}},
	}
	turn := llm.AssistantTurnResult{
		ToolCalls: []llm.AssistantToolCall{{ID: "c1", Name: "shell", Arguments: `{"command":"go test"}`}},
		Content:   `<tool_calls><tool name="readFile"><args>{"path":"x"}</args></tool></tool_calls>`,
	}
	invs, ids, reject, malformed := r.ResolveTurnInvocations(turn, nil)
	if reject || malformed != nil {
		t.Fatalf("reject=%v malformed=%v", reject, malformed)
	}
	if len(invs) != 1 || invs[0].Name != "shell" || ids[0] != "c1" {
		t.Fatalf("invs=%+v ids=%v", invs, ids)
	}
}

func TestResolveTurnInvocations_forceRejectsNative(t *testing.T) {
	r := &agentruntime.Runtime{
		Mode: "build",
		Cfg:  &config.Root{Tools: config.Tools{Legacy: true, LegacyForce: true}},
	}
	turn := llm.AssistantTurnResult{
		ToolCalls: []llm.AssistantToolCall{{ID: "c1", Name: "shell", Arguments: `{}`}},
	}
	_, _, reject, malformed := r.ResolveTurnInvocations(turn, nil)
	if !reject || malformed != nil {
		t.Fatalf("reject=%v malformed=%v", reject, malformed)
	}
}

func TestResolveTurnInvocations_legacyXML(t *testing.T) {
	r := &agentruntime.Runtime{
		Mode: "build",
		Cfg:  &config.Root{Tools: config.Tools{Legacy: true, LegacyForce: true}},
	}
	block := `<tool_calls><tool name="shell"><args>{"command":"go test"}</args></tool></tool_calls>`
	turn := llm.AssistantTurnResult{Content: block}
	invs, ids, reject, malformed := r.ResolveTurnInvocations(turn, nil)
	if reject || malformed != nil {
		t.Fatalf("reject=%v malformed=%v", reject, malformed)
	}
	if len(invs) != 1 || invs[0].Name != "shell" || ids[0] != "" {
		t.Fatalf("invs=%+v ids=%v", invs, ids)
	}
	if string(invs[0].Args) != `{"command":"go test"}` {
		t.Fatalf("args=%s", invs[0].Args)
	}
}

func TestResolveTurnInvocations_unknownToolName(t *testing.T) {
	r := &agentruntime.Runtime{
		Mode: "build",
		Cfg:  &config.Root{Tools: config.Tools{Legacy: true, LegacyForce: true}},
	}
	block := `<tool_calls><tool name="notRegistered"><args>{}</args></tool></tool_calls>`
	turn := llm.AssistantTurnResult{Content: block}
	_, _, reject, malformed := r.ResolveTurnInvocations(turn, nil)
	if reject || !errors.Is(malformed, tooling.ErrUnknownLegacyTool) {
		t.Fatalf("reject=%v malformed=%v", reject, malformed)
	}
}

func TestToolInvocationSyntaxSection(t *testing.T) {
	if got := prompt.ToolInvocationSyntaxSection(false, false, false); got != "" {
		t.Fatalf("want empty, got %q", got)
	}
	forced := prompt.ToolInvocationSyntaxSection(true, true, false)
	if !strings.Contains(forced, "<tool_calls>") || !strings.Contains(forced, "force is ON") {
		t.Fatalf("forced: %q", forced)
	}
	if strings.Contains(forced, "createPlan") {
		t.Fatalf("build forced must not include plan examples: %q", forced)
	}
	optional := prompt.ToolInvocationSyntaxSection(true, false, false)
	if !strings.Contains(optional, "Optional legacy") || !strings.Contains(optional, "native API tool_calls") {
		t.Fatalf("optional: %q", optional)
	}
}

func TestAugmentNestedCustomSystem_skipsFullTemplate(t *testing.T) {
	r := &agentruntime.Runtime{Cfg: &config.Root{Tools: config.Tools{Legacy: true, LegacyForce: true}}}
	full := "preamble\n\n## Available tools\n\nshell: ..."
	got, err := r.AugmentNestedCustomSystem(full)
	if err != nil || got != full {
		t.Fatalf("err=%v got=%q", err, got)
	}
}

func TestAugmentNestedCustomSystem_legacyOptional(t *testing.T) {
	r := &agentruntime.Runtime{Cfg: &config.Root{Tools: config.Tools{Legacy: true}}}
	got, err := r.AugmentNestedCustomSystem("You are a reviewer.")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "You are a reviewer.") || !strings.Contains(got, "<tool_calls>") {
		t.Fatalf("got=%q", got)
	}
	if strings.Contains(got, "## Available tools") {
		t.Fatalf("optional should not append tool dump: %q", got)
	}
}

func TestAugmentNestedCustomSystem_legacyForce(t *testing.T) {
	r := &agentruntime.Runtime{Cfg: &config.Root{Tools: config.Tools{Legacy: true, LegacyForce: true}}}
	got, err := r.AugmentNestedCustomSystem("Custom subagent.")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Custom subagent.") || !strings.Contains(got, "force is ON") || !strings.Contains(got, "## Available tools") {
		t.Fatalf("got=%q", got)
	}
	if !strings.Contains(got, "name: shell") {
		t.Fatalf("want build tool dump: %q", got)
	}
}
