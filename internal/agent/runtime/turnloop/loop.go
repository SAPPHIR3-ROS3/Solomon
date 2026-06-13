package turnloop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func Run(ctx context.Context, h Host) error {
	if err := h.WaitProviderReady(ctx); err != nil {
		return err
	}
	if err := h.EnsureCursorSidecar(ctx); err != nil {
		return err
	}
	stopErr := h.UserStopGeneration()
	runCtx, stopRun := context.WithCancelCause(ctx)
	defer stopRun(nil)
	setGenerationStop(stopRun)
	defer clearGenerationStop()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)
	go func() {
		for {
			select {
			case <-sigCh:
				stopRun(stopErr)
			case <-runCtx.Done():
				return
			}
		}
	}()
	out := h.Out()
	var usageTurns []llm.UsageStats
	var usageSys string
	var usageMsgs []chatstore.Message
	flushUsageStats := func() {
		if h.MachineMode() || !h.Config().UsageStatsEnabled() || len(usageTurns) == 0 {
			usageTurns = nil
			return
		}
		agg := llm.AggregateConsecutiveTurnUsage(usageTurns)
		ctxTok, usrTok, ctxEst, reasonTok, respTok, totalTok := llm.UsageTokensDisplayParts(usageSys, usageMsgs, agg, len(usageTurns))
		fmt.Fprintln(out, termcolor.UsageTokensLine(ctxTok, usrTok, reasonTok, respTok, totalTok, agg.OutputTPS, agg.TTFTSecs, agg.PromptTPS, ctxEst, agg.TurnWallSecs))
		h.MutateSession(func(s *chatstore.Session) {
			chatstore.ApplyTurnUsageDisplayToLastAssistant(s, ctxTok, usrTok, ctxEst, reasonTok, respTok, totalTok, agg.OutputTPS, agg.TTFTSecs, agg.PromptTPS, agg.TurnWallSecs)
		})
		h.PersistSessionOrLog("flushUsageStats")
		usageTurns = nil
	}
	for {
		sys, err := h.SystemPrompt(h.Config().ReasoningEffortIsNone())
		if err != nil {
			return err
		}
		legacyTools := h.LegacyToolsEnabled()
		var toolDefs []llm.ToolDef
		if !h.LegacyToolsForced() {
			tools, err := h.ToolParams()
			if err != nil {
				return err
			}
			toolDefs = llm.ToolDefsFromOpenAI(tools)
		}
		msgs, imageFiles := h.SessionMessagesSnapshot()
		turnReq := llm.TurnRequest{
			Cfg:                   h.Config(),
			Model:                 h.ModelName(),
			System:                sys,
			Messages:              msgs,
			ImageFiles:            imageFiles,
			Tools:                 toolDefs,
			ParallelToolCalls:     true,
			ForceDisableReasoning: false,
		}
		if h.Config().ReasoningEffortIsNone() {
			turnReq.ForceDisableReasoning = true
		}
		var astSeq int
		var branchKey string
		h.MutateSession(func(s *chatstore.Session) {
			astSeq = checkpoint.Bump(s)
			branchKey = s.CheckpointBranchSuffix
		})
		turnIdx := h.CITurn()
		if h.MachineMode() {
			h.CIEmit(cievents.AssistantStart(turnIdx, astSeq))
		} else {
			reasoningEff := h.Config().ReasoningEffortDisplayLabel()
			fastTag := ""
			if h.Config().FastModeEnabledForProvider(h.Provider()) {
				fastTag = " " + termcolor.WrapThinking("(fast)")
			}
			fmt.Fprintf(out, "%s%s (%s)%s: ", checkpoint.FormatLinePrefix(astSeq, branchKey), termcolor.WrapAssistant(h.ModelName()), termcolor.WrapThinking(reasoningEff), fastTag)
		}
		var legacySW *tooling.LegacyStreamWriter
		legacyOut := out
		if h.MachineMode() {
			legacyOut = io.Discard
		} else {
			legacyOut = termcolor.NewErrorLineWriter(out)
		}
		var contentOut io.Writer = legacyOut
		if legacyTools {
			allowed, err := h.AllowedToolNames()
			if err != nil {
				return err
			}
			legacySW, contentOut = h.NewLegacyStreamWriter(legacyOut, true, allowed)
		}
		streamOpts := h.StreamOptsWithRetry(h.Config().ShowThinking, out)
		if h.MachineMode() && !legacyTools {
			contentOut = io.Discard
			streamOpts = h.StreamOptsCI(turnIdx)
		} else if h.MachineMode() {
			streamOpts = h.StreamOptsCI(turnIdx)
		}
		if h.Backend() == nil {
			return fmt.Errorf("LLM backend not configured")
		}
		turn, err := h.Backend().StreamTurn(runCtx, turnReq, contentOut, streamOpts)
		if !h.MachineMode() {
			fmt.Fprintln(out)
		}
		if err != nil {
			if interruptedDuringGeneration(ctx, runCtx, err, stopErr) {
				flushUsageStats()
				logging.Log(logging.INFO_LOG_LEVEL, h.GenerationStoppedMessage())
				if !h.MachineMode() {
					h.ShowGenerationStopped(out)
				}
				return nil
			}
			if h.IsMalformedLegacyToolErr(err) {
				flushUsageStats()
				if err2 := h.HandleMalformedLegacyTool(err); err2 != nil {
					return err2
				}
				continue
			}
			flushUsageStats()
			logging.Log(logging.ERROR_LOG_LEVEL, "assistant stream failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			if h.MachineMode() {
				return h.WrapLLMErr(err)
			}
			return err
		}
		if h.MachineMode() {
			tcs := make([]chatstore.ToolCall, 0, len(turn.ToolCalls))
			for _, tc := range turn.ToolCalls {
				tcs = append(tcs, chatstore.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments})
			}
			h.CIEmit(cievents.AssistantEnd(turnIdx, turn.Content, strings.TrimSpace(turn.ReasoningText), h.ToolCallsForCI(tcs)))
			h.SetCITurn(turnIdx + 1)
		}
		if h.Config().UsageStatsEnabled() {
			usageTurns = append(usageTurns, turn.Usage)
			usageSys, usageMsgs = sys, msgs
		}
		proxyCorrection := strings.TrimSpace(turn.ProxyToolCorrection)
		turnContent := turn.Content
		if h.ExternalToolBridge() && proxyCorrection == "" {
			cleaned, fallback := h.StripCursorProxyInlineErrors(turn.Content)
			if fallback != "" {
				turnContent = cleaned
				proxyCorrection = fallback
			}
		}
		ast := chatstore.Message{Role: "assistant", Content: turnContent, ReasoningText: tooling.StripLegacyToolBlocks(strings.TrimSpace(turn.ReasoningText))}
		for _, tc := range turn.ToolCalls {
			ast.ToolCalls = append(ast.ToolCalls, chatstore.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments})
		}
		llm.PopulateAssistantTurnUsage(&ast, sys, msgs, turn.Usage)
		chatstore.BackfillAssistantUsageFromTextIfEmpty(&ast, msgs)
		h.MutateSession(func(s *chatstore.Session) {
			checkpoint.StampMsg(&ast, s, astSeq)
			s.Messages = append(s.Messages, ast)
			s.LastMessageAt = time.Now()
		})
		h.PersistSessionOrLog("assistantTurn")
		invs, toolIDs, rejectNative, malformed := h.ResolveTurnInvocations(turn, legacySW)
		if rejectNative {
			if err2 := h.HandleRejectedNativeToolCall(); err2 != nil {
				return err2
			}
			continue
		}
		if malformed != nil {
			if err2 := h.HandleMalformedLegacyTool(malformed); err2 != nil {
				return err2
			}
			continue
		}
		h.SyncLegacyToolCallsToLastAssistant(invs)
		h.PersistSessionOrLog("toolInvocations")
		if len(invs) == 0 {
			if proxyCorrection != "" {
				if err2 := h.HandleProxyToolCorrection(proxyCorrection); err2 != nil {
					return err2
				}
				continue
			}
			flushUsageStats()
			if h.MachineMode() {
				h.SetCIFinalContent(turn.Content)
			}
			if turn.Usage.PromptTokens > 0 && turn.Usage.PromptTokens >= h.CompactionThreshold() {
				deps := h.SlashDeps(runCtx)
				if h.EphemeralSession() {
					body, err := commands.SummarizeBody(deps)
					if err != nil {
						logging.Log(logging.WARNING_LOG_LEVEL, "ephemeral auto-summarize failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
						commands.PrintSystemf(out, "auto-compact: %v", err)
						return nil
					}
					h.MutateSession(func(s *chatstore.Session) {
						chatstore.ApplyCompaction(s, body, time.Now())
					})
					h.PersistSessionOrLog("ephemeralCompaction")
					commands.PrintSystem(out, "context summarized")
					continue
				}
				if err := commands.Summarize(deps); err != nil {
					logging.Log(logging.WARNING_LOG_LEVEL, "auto-compact failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
					commands.PrintSystemf(out, "auto-compact: %v", err)
				}
			}
			return nil
		}
		for i := range invs {
			if interruptedDuringGeneration(ctx, runCtx, nil, stopErr) {
				if err := appendSyntheticToolResults(h, astSeq, invs, toolIDs, i); err != nil {
					return err
				}
				flushUsageStats()
				h.ShowGenerationStopped(out)
				return nil
			}
			inv := invs[i]
			toolID := ""
			if i < len(toolIDs) {
				toolID = toolIDs[i]
			}
			var toolCpSeq int
			if h.MachineMode() {
				toolCpSeq = astSeq
				h.CIEmit(cievents.ToolStart(turnIdx, toolID, inv.Name, inv.Args))
			} else {
				toolCpSeq = h.PrintToolInvocation(i, inv.Name, inv.Args)
			}
			h.SetCurrentToolCpSeq(toolCpSeq)
			res, err := h.ExecTool(runCtx, inv)
			if interruptedDuringGeneration(ctx, runCtx, err, stopErr) {
				if err2 := appendSyntheticToolResults(h, astSeq, invs, toolIDs, i); err2 != nil {
					return err2
				}
				flushUsageStats()
				h.ShowGenerationStopped(out)
				return nil
			}
			if err != nil {
				logging.Log(logging.WARNING_LOG_LEVEL, "tool execution failed", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "err": err.Error()}})
				res = map[string]any{"error": err.Error()}
			}
			res = h.ApplyToolOutput(res, inv.Name, toolIDs[i])
			payload := toolingResultJSON(res)
			if h.MachineMode() {
				h.NoteCIToolResult(res)
				errMsg := ""
				if m, ok := res.(map[string]any); ok {
					if e, ok := m["error"].(string); ok {
						errMsg = e
					}
				}
				h.CIEmit(cievents.ToolResult(turnIdx, toolID, inv.Name, json.RawMessage(payload), errMsg))
			}
			var tm chatstore.Message
			if id := toolIDs[i]; id != "" {
				tm = chatstore.Message{Role: "tool", ToolCallID: id, Content: payload}
			} else {
				tm = chatstore.Message{Role: "user", Content: "tool_result(" + payload + ")"}
			}
			h.MutateSession(func(s *chatstore.Session) {
				checkpoint.StampMsg(&tm, s, toolCpSeq)
				s.Messages = append(s.Messages, tm)
				s.LastMessageAt = time.Now()
			})
			h.PersistSessionOrLog("toolResult")
		}
	}
}
