package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
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
	dump, err := agenttools.BuildBuildToolDump()
	if err != nil {
		return "", err
	}
	if r.MCP != nil {
		if mcpDump := strings.TrimSpace(r.MCP.ToolDump()); mcpDump != "" {
			dump = strings.TrimSpace(dump + "\n---\n" + mcpDump)
		}
	}
	return dump, nil
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
	if section := prompt.ToolInvocationSyntaxSection(r.legacyToolsEnabled(), r.legacyToolsForced(), false); section != "" {
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
	var err error
	system, err = r.AugmentNestedCustomSystem(system)
	if err != nil {
		return "", err
	}
	msgs := []chatstore.Message{{Role: "user", Content: task}}
	var transcript strings.Builder
	var usageTurns []llm.UsageStats
	var usageSys string
	var usageMsgs []chatstore.Message
	flushUsageStats := func() {
		if !r.Cfg.UsageStatsEnabled() || len(usageTurns) == 0 {
			usageTurns = nil
			return
		}
		agg := llm.AggregateConsecutiveTurnUsage(usageTurns)
		ctxTok, usrTok, ctxEst, reasonTok, respTok, totalTok := llm.UsageTokensDisplayParts(usageSys, usageMsgs, agg, len(usageTurns))
		fmt.Fprintln(r.Out, termcolor.UsageTokensLine(ctxTok, usrTok, reasonTok, respTok, totalTok, agg.OutputTPS, agg.TTFTSecs, agg.PromptTPS, ctxEst, agg.TurnWallSecs))
		usageTurns = nil
	}

	for iteration := 0; iteration < 512; iteration++ {
		dur := time.Duration(config.SubagentTimeout(r.Cfg)) * time.Minute
		roundCtx, cancel := context.WithDeadline(ctx, time.Now().Add(dur))
		turn, legacySW, err := r.streamNestedAssistant(roundCtx, system, msgs)
		cancel()
		if errors.Is(err, context.DeadlineExceeded) {
			flushUsageStats()
			logging.Log(logging.WARNING_LOG_LEVEL, "subagent round deadline exceeded", logging.LogOptions{Params: map[string]any{"timeout_min": config.SubagentTimeout(r.Cfg)}})
			if r.machineMode() {
				r.ciEmit(cievents.ErrorEvent(cievents.ExitTimeout, "subagent timeout"))
				return transcript.String(), cievents.TimeoutError(err)
			}
			sum, _ := r.summarizeNested(ctx, msgs)
			termcolor.WriteSystem(r.Out, sum+"\nSubagent paused (timeout).")
			line, _ := config.ReadPromptLine(r.promptIO(), "Continue? [y/N]: ")
			if strings.TrimSpace(strings.ToLower(line)) != "y" {
				return transcript.String(), nil
			}
			continue
		}
		if err != nil {
			flushUsageStats()
			if isMalformedLegacyToolErr(err) {
				if !r.machineMode() {
					termcolor.WriteSystem(r.Out, legacyToolScreenMessage(err))
					fmt.Fprintln(r.Out)
				}
				msgs = append(msgs, chatstore.Message{Role: "user", Content: r.toolInvocationCorrectionUserMsg()})
				continue
			}
			logging.Log(logging.ERROR_LOG_LEVEL, "subagent assistant stream failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			return transcript.String(), err
		}
		if r.Cfg.UsageStatsEnabled() {
			usageTurns = append(usageTurns, turn.Usage)
			usageSys, usageMsgs = system, msgs
		}
		if rt := strings.TrimSpace(turn.ReasoningText); rt != "" {
			transcript.WriteString(rt)
			transcript.WriteByte('\n')
		}
		transcript.WriteString(turn.Content)
		transcript.WriteByte('\n')
		ast := chatstore.Message{Role: "assistant", Content: turn.Content, ReasoningText: tooling.StripLegacyToolBlocks(strings.TrimSpace(turn.ReasoningText))}
		for _, tc := range turn.ToolCalls {
			ast.ToolCalls = append(ast.ToolCalls, chatstore.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments})
		}
		msgs = append(msgs, ast)
		var invs []tooling.Invocation
		var toolIDs []string
		invs, toolIDs, rejectNative, malformed := r.ResolveTurnInvocations(turn, legacySW)
		if rejectNative {
			if !r.machineMode() {
				termcolor.WriteSystem(r.Out, "Legacy tools force: native API tool_calls were ignored. Use <tool_calls> XML in assistant text.")
				fmt.Fprintln(r.Out)
			}
			msgs = append(msgs, chatstore.Message{Role: "user", Content: legacyNativeToolRejectedUserMsg})
			continue
		}
		if malformed != nil {
			if !r.machineMode() {
				termcolor.WriteSystem(r.Out, legacyToolScreenMessage(malformed))
				fmt.Fprintln(r.Out)
			}
			msgs = append(msgs, chatstore.Message{Role: "user", Content: r.toolInvocationCorrectionUserMsg()})
			continue
		}
		if len(invs) == 0 {
			flushUsageStats()
			return transcript.String(), nil
		}
		for i, inv := range invs {
			r.printToolLine(0, "", inv.Name, inv.Args)
			for _, line := range formatToolPlainLines(inv.Name, inv.Args) {
				transcript.WriteString(line + "\n")
			}
			res, err := r.execTool(ctx, inv)
			if err != nil {
				logging.Log(logging.WARNING_LOG_LEVEL, "nested tool execution failed", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "err": err.Error()}})
				res = map[string]any{"error": err.Error()}
			}
			res = r.applyToolOutput(res, inv.Name, toolIDs[i])
			b, err := json.Marshal(res)
			if err != nil {
				b = []byte(`{"error":"marshal"}`)
			}
			payload := string(b)
			if id := toolIDs[i]; id != "" {
				msgs = append(msgs, chatstore.Message{Role: "tool", ToolCallID: id, Content: payload})
			} else {
				msgs = append(msgs, chatstore.Message{Role: "user", Content: "tool_result(" + payload + ")"})
			}
		}
	}
	flushUsageStats()
	return transcript.String(), nil
}

func (r *Runtime) streamNestedAssistant(ctx context.Context, system string, msgs []chatstore.Message) (llm.AssistantTurnResult, *tooling.LegacyStreamWriter, error) {
	var toolDefs []llm.ToolDef
	if !r.legacyToolsForced() {
		toolParams, err := agenttools.NativeToolParams("build")
		if err != nil {
			return llm.AssistantTurnResult{}, nil, err
		}
		if r.MCP != nil {
			toolParams = append(toolParams, r.MCP.OpenAITools()...)
		}
		toolDefs = llm.ToolDefsFromOpenAI(toolParams)
	}
	turnReq := llm.TurnRequest{
		Cfg:                   r.Cfg,
		Model:                 r.Model,
		System:                system,
		Messages:              msgs,
		ImageFiles:            r.Session.ImageFiles,
		Tools:                 toolDefs,
		ParallelToolCalls:     true,
		ForceDisableReasoning: true,
	}
	fmt.Fprintf(r.Out, "%s ", termcolor.WrapAssistant(r.Model+"(subagent):"))
	if r.Backend == nil {
		return llm.AssistantTurnResult{}, nil, fmt.Errorf("LLM backend not configured")
	}
	var legacySW *tooling.LegacyStreamWriter
	var contentOut io.Writer = termcolor.NewErrorLineWriter(r.Out)
	if r.legacyToolsEnabled() {
		allowed, err := r.allowedToolNamesForMode("build")
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
	turn, err := r.Backend.StreamTurn(ctx, turnReq, contentOut, r.streamOptsWithRetry(r.Cfg.ShowThinking, r.Out))
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "nested subagent stream failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return turn, legacySW, err
	}
	fmt.Fprintln(r.Out)
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
