package tools

import (
	"context"
	"encoding/json"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureSubagent(sysPromptPath, task string) {}

type subagentArgs struct {
	SysPromptPath string `json:"sysPromptPath"`
	Task          string `json:"task"`
}

func subagentOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("subagent", "Run a nested agent with system prompt from file and task string.", map[string]any{
		"sysPromptPath": map[string]any{"type": "string", "description": "Path to system prompt file"},
		"task":          map[string]any{"type": "string", "description": "Concrete task for the nested run"},
	}, []string{"sysPromptPath", "task"})
}

func appendSubagentDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureSubagent)
	if err != nil {
		return err
	}
	b.addBlock("subagent", "Run a nested agent with system prompt from file and task string.", sig)
	return nil
}

func execSubagent(ctx context.Context, env *Env, raw json.RawMessage) (any, error) {
	var a subagentArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	p := resolveProjectPath(env.ProjRoot, a.SysPromptPath)
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	sys := string(b)
	prev := env.CurrentMode()
	env.SetMode("build")
	out, err := env.RunNestedWithSystem(ctx, sys, a.Task)
	env.SetMode(prev)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "output": out}, nil
}
