package agentruntime

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func (r *Runtime) runNested(ctx context.Context, task string) (string, error) {
	sys, err := r.systemPrompt(true)
	if err != nil {
		return "", err
	}
	return r.runNestedWithSystem(ctx, sys, task)
}

const nestedFullSystemMarker = "## Available tools"

func (r *Runtime) buildNestedToolDump() (string, error) {
	return agenttools.BuildDeferredToolDump()
}

func (r *Runtime) AugmentNestedCustomSystem(system string) (string, error) {
	if strings.Contains(system, nestedFullSystemMarker) {
		return system, nil
	}
	system = strings.TrimSpace(system)
	if r.externalToolBridge() {
		return system, nil
	}
	if !r.legacyToolsEnabled() {
		return system, nil
	}
	blocks := []string{system}
	if section := prompt.ToolInvocationSyntaxSection(r.legacyToolsEnabled(), r.legacyToolsForced(), r.Session != nil && r.Session.PlanningActive); section != "" {
		blocks = append(blocks, section)
	}
	if r.legacyToolsForced() {
		dump, err := r.buildNestedToolDump()
		if err != nil {
			return "", err
		}
		blocks = append(blocks, "## Available tools\n\n"+dump)
	}
	return strings.Join(blocks, "\n\n"), nil
}

func (r *Runtime) runNestedWithSystem(ctx context.Context, system, task string) (string, error) {
	res, err := r.runNestedWithConfig(ctx, NestedRunConfig{
		SysPrompt:  system,
		Task:       task,
		SpawnTime:  time.Now().UTC(),
		Origin:     chatstore.SubOriginParent,
		ProjectHex: r.ProjHex,
		ToolCall:   chatstore.ToolCall{Name: "subagent", Arguments: `{"task":""}`},
	})
	if err != nil {
		return "", err
	}
	return res.Output, nil
}

func (r *Runtime) streamNestedAssistant(ctx context.Context, out io.Writer, system string, msgs []chatstore.Message, imageFiles map[int]string, forceDisableReasoning bool, cfg NestedRunConfig) (llm.AssistantTurnResult, *tooling.LegacyStreamWriter, error) {
	var toolDefs []llm.ToolDef
	if !r.legacyToolsForced() {
		toolParams, err := agenttools.NativeToolParams("agent")
		if err != nil {
			return llm.AssistantTurnResult{}, nil, err
		}
		if r.MCP != nil {
			toolParams = append(toolParams, r.MCP.OpenAITools()...)
		}
		toolDefs = llm.ToolDefsFromOpenAI(toolParams)
	}
	model, backend, label, err := r.nestedLLMTarget(ctx, cfg)
	if err != nil {
		return llm.AssistantTurnResult{}, nil, err
	}
	turnReq := llm.TurnRequest{
		Cfg:                   r.Cfg,
		Model:                 model,
		System:                system,
		Messages:              msgs,
		ImageFiles:            imageFiles,
		Tools:                 toolDefs,
		ParallelToolCalls:     true,
		ForceDisableReasoning: forceDisableReasoning,
	}
	fmt.Fprintf(out, "%s ", termcolor.WrapAssistant(label+":"))
	var legacySW *tooling.LegacyStreamWriter
	var contentOut io.Writer = termcolor.NewErrorLineWriter(out)
	if r.legacyToolsEnabled() {
		allowed, err := r.allowedToolNames()
		if err != nil {
			return llm.AssistantTurnResult{}, nil, err
		}
		if r.MCP != nil {
			for _, t := range r.MCP.OpenAITools() {
				if t.OfFunction == nil {
					continue
				}
				name := t.OfFunction.Function.Name
				if name != "" {
					allowed[name] = struct{}{}
				}
			}
		}
		legacySW, contentOut = newLegacyStreamWriter(contentOut, true, allowed)
	}
	turn, err := backend.StreamTurn(ctx, turnReq, contentOut, r.streamOptsWithRetry(r.Cfg.ShowThinking, out))
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "nested subagent stream failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return turn, legacySW, err
	}
	fmt.Fprintln(out)
	return turn, legacySW, nil
}

func (r *Runtime) summarizeNested(ctx context.Context, msgs []chatstore.Message) (string, error) {
	var sb strings.Builder
	for _, m := range msgs {
		sb.WriteString(m.Role + ": " + m.Content + "\n")
	}
	if r.Backend == nil {
		return "", fmt.Errorf("LLM backend not configured")
	}
	sys := prompt.SystemWithNoThink(true, "Briefly summarize the following conversation turns.")
	text, err := r.Backend.CompleteText(ctx, llm.SimpleCompletionRequest{
		Cfg:                   r.Cfg,
		Model:                 r.Model,
		System:                sys,
		User:                  sb.String(),
		ForceDisableReasoning: true,
	})
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "nested summarize completion failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		logging.Log(logging.WARNING_LOG_LEVEL, "nested summarize: empty response")
		return "", fmt.Errorf("no summary choices")
	}
	return strings.TrimSpace(text), nil
}
