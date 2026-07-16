package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func (r *Runtime) beginNestedRun(ctx context.Context, cfg NestedRunConfig) (NestedRunConfig, *chatstore.SubSession, string, error) {
	if err := r.resolveSubagentRole(&cfg); err != nil {
		return cfg, nil, "", err
	}
	sess, id, err := r.prepareSubSession(ctx, cfg)
	if err != nil {
		return cfg, nil, "", err
	}
	if strings.TrimSpace(cfg.RoleModel) == "" && sess != nil {
		cfg.RoleProvider, cfg.RoleModel = subagentRoleFromSession(sess)
	}
	if strings.TrimSpace(cfg.RoleProvider) != "" && strings.TrimSpace(cfg.RoleModel) != "" {
		if err := r.resolveSubagentRole(&cfg); err != nil {
			return cfg, nil, "", err
		}
	}
	return cfg, sess, id, nil
}

func (r *Runtime) runNestedWithConfig(ctx context.Context, cfg NestedRunConfig) (NestedRunResult, error) {
	cfg, sess, id, err := r.beginNestedRun(ctx, cfg)
	if err != nil {
		return NestedRunResult{}, err
	}
	system := cfg.SysPrompt
	if system == "" {
		p := cfg.SysPromptPath
		if p == "" && sess != nil {
			p = sess.SysPromptPath
		}
		if p != "" {
			p = agenttools.ResolveSysPromptPath(r.ProjRoot, p)
			b, err := os.ReadFile(p)
			if err != nil {
				return NestedRunResult{}, err
			}
			system = string(b)
			if r.Instructions != nil {
				if merged, err := r.mergeSystemWithInstructions(system); err == nil {
					system = merged
				}
			}
		}
	}
	system, err = r.AugmentNestedCustomSystem(system)
	if err != nil {
		return NestedRunResult{}, err
	}
	if !r.EphemeralSession {
		if err := r.persistSubSession(sess); err != nil {
			return NestedRunResult{}, err
		}
		_ = globalSubagentRegistry.upsertActiveEntry(r.activeEntryFor(sess))
	}
	prevNested := r.getNestedState()
	r.setNestedState(&activeNestedState{
		subchatID:    id,
		origin:       sess.Origin,
		parentChatID: sess.ParentChatID,
		projectHex:   sess.ProjectHex,
	})
	defer r.setNestedState(prevNested)

	quiet := cfg.RunInBackground
	streamOut := r.Out
	if quiet {
		streamOut = io.Discard
	}

	msgs := append([]chatstore.Message(nil), sess.Messages...)
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
		fmt.Fprintln(streamOut, termcolor.UsageTokensLine(ctxTok, usrTok, reasonTok, respTok, totalTok, agg.OutputTPS, agg.TTFTSecs, agg.PromptTPS, ctxEst, agg.TurnWallSecs))
		if !r.EphemeralSession {
			chatstore.ApplyTurnUsageDisplayToLastSubAssistant(sess, ctxTok, usrTok, ctxEst, reasonTok, respTok, totalTok, agg.OutputTPS, agg.TTFTSecs, agg.PromptTPS, agg.TurnWallSecs)
			_ = r.persistSubSession(sess)
		}
		usageTurns = nil
	}
	effort, forceDisable := r.Cfg.EffectiveSubagentReasoningEffort(cfg.ReasoningEffort)
	if strings.TrimSpace(cfg.ReasoningEffort) == "" && sess.ReasoningEffort != "" {
		effort, forceDisable = r.Cfg.EffectiveSubagentReasoningEffort(sess.ReasoningEffort)
	}
	for iteration := 0; iteration < 512; iteration++ {
		dur := time.Duration(config.SubagentTimeout(r.Cfg)) * time.Minute
		roundCtx, cancel := context.WithDeadline(ctx, time.Now().Add(dur))
		turn, legacySW, err := r.streamNestedAssistant(roundCtx, streamOut, system, msgs, r.subSessionImageFiles(sess), forceDisable, effort, cfg)
		cancel()
		if errors.Is(err, context.DeadlineExceeded) {
			flushUsageStats()
			logging.Log(logging.WARNING_LOG_LEVEL, "subagent round deadline exceeded", logging.LogOptions{Params: map[string]any{"timeout_min": config.SubagentTimeout(r.Cfg)}})
			if r.machineMode() {
				r.ciEmit(cievents.ErrorEvent(cievents.ExitTimeout, "subagent timeout"))
				sess.Status = chatstore.SubStatusPaused
				_ = r.persistSubSession(sess)
				return NestedRunResult{Output: transcript.String(), SubchatID: id, Status: sess.Status}, cievents.TimeoutError(err)
			}
			if quiet {
				sess.Status = chatstore.SubStatusPaused
				sess.Messages = msgs
				_ = r.persistSubSession(sess)
				return NestedRunResult{Output: transcript.String(), SubchatID: id, Status: sess.Status}, nil
			}
			sum, _ := r.summarizeNested(ctx, msgs)
			termcolor.WriteSystem(r.Out, sum+"\nSubagent paused (timeout).")
			line, _ := config.ReadPromptLine(r.promptIO(), "Continue? [y/N]: ")
			if strings.TrimSpace(strings.ToLower(line)) != "y" {
				sess.Status = chatstore.SubStatusPaused
				sess.Messages = msgs
				_ = r.persistSubSession(sess)
				return NestedRunResult{Output: transcript.String(), SubchatID: id, Status: sess.Status}, nil
			}
			continue
		}
		if err != nil {
			flushUsageStats()
			if isMalformedLegacyToolErr(err) {
				if !r.machineMode() && !quiet {
					termcolor.WriteSystem(r.Out, legacyToolScreenMessage(err))
					fmt.Fprintln(r.Out)
				}
				msgs = append(msgs, chatstore.Message{Role: "user", Content: r.toolInvocationCorrectionUserMsg()})
				continue
			}
			logging.Log(logging.ERROR_LOG_LEVEL, "subagent assistant stream failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			sess.Status = chatstore.SubStatusPaused
			sess.Messages = msgs
			_ = r.persistSubSession(sess)
			return NestedRunResult{Output: transcript.String(), SubchatID: id, Status: sess.Status}, err
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
		sess.Messages = msgs
		sess.LastMessageAt = time.Now().UTC()
		if !r.EphemeralSession {
			_ = r.persistSubSession(sess)
		}
		invs, toolIDs, rejectNative, malformed := r.ResolveTurnInvocations(turn, legacySW)
		if rejectNative {
			if !r.machineMode() && !quiet {
				termcolor.WriteSystem(r.Out, "Legacy tools force: native API tool_calls were ignored. Use <tool_calls> XML in assistant text.")
				fmt.Fprintln(r.Out)
			}
			msgs = append(msgs, chatstore.Message{Role: "user", Content: legacyNativeToolRejectedUserMsg})
			continue
		}
		if malformed != nil {
			if !r.machineMode() && !quiet {
				termcolor.WriteSystem(r.Out, legacyToolScreenMessage(malformed))
				fmt.Fprintln(r.Out)
			}
			msgs = append(msgs, chatstore.Message{Role: "user", Content: r.toolInvocationCorrectionUserMsg()})
			continue
		}
		if len(invs) == 0 {
			flushUsageStats()
			sess.Status = chatstore.SubStatusDone
			sess.Messages = msgs
			_ = r.persistSubSession(sess)
			_ = globalSubagentRegistry.removeActiveEntry(id)
			return NestedRunResult{Output: transcript.String(), SubchatID: id, Status: sess.Status}, nil
		}
		for i, inv := range invs {
			inv.ToolCallID = toolIDs[i]
			if !quiet {
				r.printToolLine(0, "", inv.Name, inv.Args)
			}
			for _, line := range formatToolPlainLines(inv.Name, inv.Args) {
				transcript.WriteString(line + "\n")
			}
			res, err := r.execToolNestedAware(ctx, inv)
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
		sess.Messages = msgs
		if !r.EphemeralSession {
			_ = r.persistSubSession(sess)
		}
	}
	flushUsageStats()
	sess.Status = chatstore.SubStatusDone
	_ = r.persistSubSession(sess)
	return NestedRunResult{Output: transcript.String(), SubchatID: id, Status: sess.Status}, nil
}
