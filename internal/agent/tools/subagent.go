package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureSubagent(sysPromptPath, task string) {}

type SubagentArgs struct {
	SysPromptPath   string `json:"sysPromptPath"`
	Task            string `json:"task"`
	Resume          string `json:"resume,omitempty"`
	RunInBackground bool   `json:"run_in_background,omitempty"`
	ReasoningEffort string `json:"reasoningEffort,omitempty"`
}

type subagentArgs = SubagentArgs

type subagentResult struct {
	OK        bool   `json:"ok"`
	Output    string `json:"output,omitempty"`
	SubchatID string `json:"subchatId,omitempty"`
	Status    string `json:"status,omitempty"`
	Error     string `json:"error,omitempty"`
}

func subagentPromptTemplatesDir() string {
	dir, err := paths.PromptTemplatesDir()
	if err != nil {
		return "~/.solomon/prompts/templates"
	}
	return dir
}

func subagentToolSummary() string {
	dir := subagentPromptTemplatesDir()
	return fmt.Sprintf("Run a nested agent with system prompt from file and task string (native tool_call only — not via orchestrate). Solomon templates: %s/<name>.tmpl (agent, chat, …). Default: synchronous (parent waits for output). Set run_in_background true for async: returns subchatId immediately while the subagent keeps running.", dir)
}

func subagentSysPromptPathDescription() string {
	dir := subagentPromptTemplatesDir()
	return fmt.Sprintf("Path to system prompt file; editable templates under %s (e.g. %s/agent.tmpl)", dir, dir)
}

func subagentOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("subagent", subagentToolSummary(), map[string]any{
		"sysPromptPath": map[string]any{"type": "string", "description": subagentSysPromptPathDescription()},
		"task":          map[string]any{"type": "string", "description": "Concrete task for the nested run"},
		"resume":        map[string]any{"type": "string", "description": "Subchat ID to resume"},
		"run_in_background": map[string]any{"type": "boolean", "description": "Async: true = do not block parent (returns subchatId, status running). False/omit = sync: wait until done (returns output, status done)."},
		"reasoningEffort": map[string]any{"type": "string", "description": "Override reasoning: none, low, medium, high"},
	}, []string{"sysPromptPath", "task"})
}

func appendSubagentDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureSubagent)
	if err != nil {
		return err
	}
	b.addBlock("subagent", subagentToolSummary(), sig)
	return nil
}

func execSubagent(ctx context.Context, env *Env, raw json.RawMessage) (any, error) {
	a, err := parseSubagentArgs(raw)
	if err != nil {
		return nil, err
	}
	if env.RunSubagent == nil {
		return nil, fmt.Errorf("subagent runtime unavailable")
	}
	var sys string
	if a.SysPromptPath != "" {
		p := ResolveSysPromptPath(env.ProjRoot, a.SysPromptPath)
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		sys = string(b)
		if env.MergeInstructionBlock != nil {
			merged, err := env.MergeInstructionBlock(sys)
			if err != nil {
				return nil, err
			}
			sys = merged
		}
	}
	if a.Resume == "" && strings.TrimSpace(a.Task) == "" {
		return nil, fmt.Errorf("task is required for new subagent")
	}
	tc := chatstore.ToolCall{Name: "subagent", Arguments: string(raw)}
	if env.ParentToolCallID != "" {
		tc.ID = env.ParentToolCallID
	}
	prev := env.CurrentMode()
	env.SetMode("agent")
	res, err := env.RunSubagent(ctx, SubagentRequest{
		SysPromptPath:   a.SysPromptPath,
		SysPrompt:       sys,
		Task:            a.Task,
		Resume:          a.Resume,
		RunInBackground: a.RunInBackground,
		ReasoningEffort: a.ReasoningEffort,
		ToolCall:        tc,
	})
	env.SetMode(prev)
	if err != nil {
		return subagentResult{Error: err.Error()}, nil
	}
	return subagentResult{
		OK:        true,
		Output:    res.Output,
		SubchatID: res.SubchatID,
		Status:    res.Status,
	}, nil
}

func ParseSubagentArgsForTest(raw json.RawMessage) (SubagentArgs, error) {
	return parseSubagentArgs(raw)
}

func parseSubagentArgs(raw json.RawMessage) (SubagentArgs, error) {
	var a SubagentArgs
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return a, err
	}
	if v, ok := m["sysPromptPath"]; ok {
		_ = json.Unmarshal(v, &a.SysPromptPath)
	}
	if v, ok := m["task"]; ok {
		_ = json.Unmarshal(v, &a.Task)
	}
	if v, ok := m["resume"]; ok {
		_ = json.Unmarshal(v, &a.Resume)
	}
	if v, ok := m["reasoningEffort"]; ok {
		_ = json.Unmarshal(v, &a.ReasoningEffort)
	}
	if b, ok := parseJSONBool(m["run_in_background"]); ok {
		a.RunInBackground = b
	}
	return a, nil
}

func parseJSONBool(raw json.RawMessage) (bool, bool) {
	if len(raw) == 0 {
		return false, false
	}
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		return b, true
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return false, false
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}

