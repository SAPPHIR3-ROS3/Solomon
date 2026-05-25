package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/agent/cievents"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/title"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"
)

func flushWriter(w io.Writer) {
	if f, ok := w.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
}

func showGenerationStopped(out io.Writer) {
	termcolor.WriteSystem(out, "["+cliMsgGenerationStopped+"]")
	flushWriter(out)
}

func (r *Runtime) onUserMessage(ctx context.Context, line string, fromReadline bool) error {
	return r.onUserMessageWithAPIContent(ctx, line, "", fromReadline)
}

func (r *Runtime) onUserMessageWithAPIContent(ctx context.Context, line string, apiContent string, fromReadline bool) error {
	clean, _ := parseMultilineControlRunes(line)
	line = trimMessageEdges(clean)
	apiContent = trimMessageEdges(apiContent)
	if config.NeedsOnboard(r.Cfg) || r.Prov == nil {
		return fmt.Errorf("config not set up; use /onboard")
	}
	if r.ReplShellFirst {
		if strings.HasPrefix(line, "!") {
			line = trimMessageEdges(strings.TrimPrefix(line, "!"))
			if line == "" {
				return nil
			}
		} else {
			if line == "" {
				return nil
			}
			return r.runUserShellLine(ctx, line)
		}
	} else if strings.HasPrefix(line, "!") {
		cmd := trimMessageEdges(strings.TrimPrefix(line, "!"))
		if cmd == "" {
			return nil
		}
		return r.runUserShellLine(ctx, cmd)
	}
	var um chatstore.Message
	var firstUserLine string
	r.mutateSession(func(s *chatstore.Session) {
		if !r.EphemeralSession {
			r.markSessionFileCreated()
			if s.ID == "" && len(s.Messages) == 0 {
				s.ID = chatstore.NewPlaceholderChatID(time.Now())
			}
		}
		if r.EphemeralSession && s.Title == "" && len(s.Messages) == 0 {
			tSlug := title.NormalizeSlug(title.FallbackFromWords(line))
			s.Title = tSlug
			s.ID = chatstore.ChatIDHex(tSlug, s.CreatedAt)
			firstUserLine = strings.TrimSpace(line)
		}
		seq := checkpoint.Bump(s)
		um = chatstore.Message{Role: "user", Content: line, APIContent: apiContent}
		checkpoint.StampMsg(&um, s, seq)
		s.Messages = append(s.Messages, um)
		chatstore.RepairSessionMalformedImages(s)
		s.LastMessageAt = time.Now()
		s.LastUserMessageAt = time.Now()
	})
	if firstUserLine != "" {
		go r.refineEphemeralTitle(ctx, firstUserLine)
	}
	if !fromReadline && !r.machineMode() {
		echoLine := termcolor.ColorizeImgTags(line)
		cpPref := checkpoint.FormatLinePrefix(um.CheckpointSeq, um.CheckpointBranchKey)
		youLbl := termcolor.WrapUser("You:")
		fmt.Fprintf(r.Out, "%s%s %s\n", cpPref, youLbl, echoLine)
	}
	if err := r.persistSession(); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "persist session failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return err
	}
	if err := r.runAgentTurns(ctx); err != nil {
		return err
	}
	var deferTitle bool
	r.mutateSession(func(s *chatstore.Session) {
		deferTitle = !r.EphemeralSession && chatstore.IsPlaceholderChatID(s.ID)
	})
	if deferTitle {
		r.scheduleDeferredChatTitleFinalize(ctx)
	}
	return nil
}

func (r *Runtime) runAgentTurns(ctx context.Context) error {
	runCtx, stopRun := context.WithCancelCause(ctx)
	defer stopRun(nil)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)
	go func() {
		for {
			select {
			case <-sigCh:
				stopRun(errUserStopGeneration)
			case <-runCtx.Done():
				return
			}
		}
	}()
	var usageTurns []llm.UsageStats
	var usageSys string
	var usageMsgs []chatstore.Message
	flushUsageStats := func() {
		if r.machineMode() || !r.Cfg.UsageStatsEnabled() || len(usageTurns) == 0 {
			usageTurns = nil
			return
		}
		agg := llm.AggregateConsecutiveTurnUsage(usageTurns)
		ctxTok, usrTok, ctxEst, reasonTok, respTok, totalTok := llm.UsageTokensDisplayParts(usageSys, usageMsgs, agg, len(usageTurns))
		fmt.Fprintln(r.Out, termcolor.UsageTokensLine(ctxTok, usrTok, reasonTok, respTok, totalTok, agg.OutputTPS, agg.TTFTSecs, agg.PromptTPS, ctxEst, agg.TurnWallSecs))
		r.mutateSession(func(s *chatstore.Session) {
			chatstore.ApplyTurnUsageDisplayToLastAssistant(s, ctxTok, usrTok, ctxEst, reasonTok, respTok, totalTok, agg.OutputTPS, agg.TTFTSecs, agg.PromptTPS, agg.TurnWallSecs)
		})
		_ = r.persistSession()
		usageTurns = nil
	}
	for {
		sys, err := r.systemPrompt(r.Cfg.ReasoningEffortIsNone())
		if err != nil {
			return err
		}
		legacyTools := r.legacyToolsEnabled()
		var toolDefs []llm.ToolDef
		if !r.legacyToolsForced() {
			tools, err := r.toolParams()
			if err != nil {
				return err
			}
			toolDefs = llm.ToolDefsFromOpenAI(tools)
		}
		msgs, imageFiles := r.sessionMessagesSnapshot()
		turnReq := llm.TurnRequest{
			Cfg:                   r.Cfg,
			Model:                 r.Model,
			System:                sys,
			Messages:              msgs,
			ImageFiles:            imageFiles,
			Tools:                 toolDefs,
			ParallelToolCalls:     true,
			ForceDisableReasoning: false,
		}
		if r.Cfg.ReasoningEffortIsNone() {
			turnReq.ForceDisableReasoning = true
		}
		var astSeq int
		var branchKey string
		r.mutateSession(func(s *chatstore.Session) {
			astSeq = checkpoint.Bump(s)
			branchKey = s.CheckpointBranchSuffix
		})
		turnIdx := r.ciTurn
		if r.machineMode() {
			r.ciEmit(cievents.AssistantStart(turnIdx, astSeq))
		} else {
			reasoningEff := "none"
			if lbl := r.Cfg.ReasoningEffortLabel(); lbl != "" {
				reasoningEff = lbl
			}
			fmt.Fprintf(r.Out, "%s%s (%s): ", checkpoint.FormatLinePrefix(astSeq, branchKey), termcolor.WrapAssistant(r.Model), termcolor.WrapThinking(reasoningEff))
		}
		var legacySW *tooling.LegacyStreamWriter
		legacyOut := r.Out
		if r.machineMode() {
			legacyOut = io.Discard
		}
		var contentOut io.Writer = legacyOut
		if legacyTools {
			allowed, err := r.allowedToolNames()
			if err != nil {
				return err
			}
			legacySW, contentOut = newLegacyStreamWriter(legacyOut, true, allowed, checkpoint.FormatLinePrefix(astSeq, branchKey))
		}
		streamOpts := r.streamOptsWithRetry(r.Cfg.ShowThinking, r.Out)
		if r.machineMode() && !legacyTools {
			contentOut = io.Discard
			streamOpts = r.streamOptsCI(turnIdx)
		} else if r.machineMode() {
			streamOpts = r.streamOptsCI(turnIdx)
		}
		if r.Backend == nil {
			return fmt.Errorf("LLM backend not configured")
		}
		turn, err := r.Backend.StreamTurn(runCtx, turnReq, contentOut, streamOpts)
		if !r.machineMode() {
			fmt.Fprintln(r.Out)
		}
		if err != nil {
			if interruptedDuringGeneration(ctx, runCtx, err) {
				flushUsageStats()
				logging.Log(logging.INFO_LOG_LEVEL, cliMsgGenerationStopped)
				if !r.machineMode() {
					showGenerationStopped(r.Out)
				}
				return nil
			}
			if isMalformedLegacyToolErr(err) {
				flushUsageStats()
				if err2 := r.handleMalformedLegacyTool(err); err2 != nil {
					return err2
				}
				continue
			}
			flushUsageStats()
			logging.Log(logging.ERROR_LOG_LEVEL, "assistant stream failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			if r.machineMode() {
				return r.wrapLLMErr(err)
			}
			return err
		}
		if r.machineMode() {
			tcs := make([]chatstore.ToolCall, 0, len(turn.ToolCalls))
			for _, tc := range turn.ToolCalls {
				tcs = append(tcs, chatstore.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments})
			}
			r.ciEmit(cievents.AssistantEnd(turnIdx, turn.Content, strings.TrimSpace(turn.ReasoningText), toolCallsCI(tcs)))
			r.ciTurn++
		}
		if r.Cfg.UsageStatsEnabled() {
			usageTurns = append(usageTurns, turn.Usage)
			usageSys, usageMsgs = sys, msgs
		}
		ast := chatstore.Message{Role: "assistant", Content: turn.Content, ReasoningText: tooling.StripLegacyToolBlocks(strings.TrimSpace(turn.ReasoningText))}
		for _, tc := range turn.ToolCalls {
			ast.ToolCalls = append(ast.ToolCalls, chatstore.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments})
		}
		llm.PopulateAssistantTurnUsage(&ast, sys, msgs, turn.Usage)
		chatstore.BackfillAssistantUsageFromTextIfEmpty(&ast, msgs)
		r.mutateSession(func(s *chatstore.Session) {
			checkpoint.StampMsg(&ast, s, astSeq)
			s.Messages = append(s.Messages, ast)
			s.LastMessageAt = time.Now()
		})
		_ = r.persistSession()
		var invs []tooling.Invocation
		var toolIDs []string
		invs, toolIDs, rejectNative, malformed := r.ResolveTurnInvocations(turn, legacySW)
		if rejectNative {
			if err2 := r.handleRejectedNativeToolCall(); err2 != nil {
				return err2
			}
			continue
		}
		if malformed != nil {
			if err2 := r.handleMalformedLegacyTool(malformed); err2 != nil {
				return err2
			}
			continue
		}
		r.syncLegacyToolCallsToLastAssistant(invs)
		_ = r.persistSession()
		if len(invs) == 0 {
			flushUsageStats()
			if r.machineMode() {
				r.ciFinalContent = turn.Content
			}
			if turn.Usage.PromptTokens > 0 && turn.Usage.PromptTokens >= r.CompactionThresholdTokens {
				deps := r.slashDeps(runCtx)
				if r.EphemeralSession {
					body, err := commands.SummarizeBody(deps)
					if err != nil {
						logging.Log(logging.WARNING_LOG_LEVEL, "ephemeral auto-summarize failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
						commands.PrintSystemf(r.Out, "auto-compact: %v", err)
						return nil
					}
					r.mutateSession(func(s *chatstore.Session) {
						s.Messages = []chatstore.Message{{Role: "assistant", Content: body}}
						s.MainOrphans = nil
						s.CheckpointBranchSuffix = ""
						s.ForkChildCount = nil
						s.CheckpointLast = -1
						s.LastCommitOID = ""
						s.LastMessageAt = time.Now()
						chatstore.RepairSessionMalformedImages(s)
					})
					_ = r.persistSession()
					commands.PrintSystem(r.Out, "context summarized")
					continue
				}
				if err := commands.Summarize(deps); err != nil {
					logging.Log(logging.WARNING_LOG_LEVEL, "auto-compact failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
					commands.PrintSystemf(r.Out, "auto-compact: %v", err)
				}
			}
			return nil
		}
		for i := range invs {
			if interruptedDuringGeneration(ctx, runCtx, nil) {
				if err := r.appendSyntheticToolResults(astSeq, invs, toolIDs, i); err != nil {
					return err
				}
				flushUsageStats()
				showGenerationStopped(r.Out)
				return nil
			}
			inv := invs[i]
			toolID := ""
			if i < len(toolIDs) {
				toolID = toolIDs[i]
			}
			if r.machineMode() {
				r.ciEmit(cievents.ToolStart(turnIdx, toolID, inv.Name, inv.Args))
			} else if legacySW == nil || !legacySW.DisplayRendered() {
				r.printToolLine(astSeq, branchKey, inv.Name, inv.Args)
			}
			res, err := r.execTool(runCtx, inv)
			if interruptedDuringGeneration(ctx, runCtx, err) {
				if err2 := r.appendSyntheticToolResults(astSeq, invs, toolIDs, i); err2 != nil {
					return err2
				}
				flushUsageStats()
				showGenerationStopped(r.Out)
				return nil
			}
			if err != nil {
				logging.Log(logging.WARNING_LOG_LEVEL, "tool execution failed", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "err": err.Error()}})
				res = map[string]any{"error": err.Error()}
			}
			res = r.applyToolOutput(res, inv.Name, toolIDs[i])
			payload := toolingResultJSON(res)
			if r.machineMode() {
				r.noteCIToolResult(res)
				errMsg := ""
				if m, ok := res.(map[string]any); ok {
					if e, ok := m["error"].(string); ok {
						errMsg = e
					}
				}
				r.ciEmit(cievents.ToolResult(turnIdx, toolID, inv.Name, json.RawMessage(payload), errMsg))
			}
			var tm chatstore.Message
			if id := toolIDs[i]; id != "" {
				tm = chatstore.Message{Role: "tool", ToolCallID: id, Content: payload}
			} else {
				tm = chatstore.Message{Role: "user", Content: "tool_result(" + payload + ")"}
			}
			r.mutateSession(func(s *chatstore.Session) {
				checkpoint.StampMsg(&tm, s, astSeq)
				s.Messages = append(s.Messages, tm)
				s.LastMessageAt = time.Now()
			})
			_ = r.persistSession()
		}
	}
}

func interruptedDuringGeneration(parent, runCtx context.Context, opErr error) bool {
	if errors.Is(context.Cause(runCtx), errUserStopGeneration) {
		return true
	}
	if opErr == nil {
		return false
	}
	return parent.Err() == nil && errors.Is(runCtx.Err(), context.Canceled) && errors.Is(opErr, context.Canceled)
}

func (r *Runtime) appendSyntheticToolResults(astSeq int, invs []tooling.Invocation, toolIDs []string, start int) error {
	payload := toolingResultJSON(map[string]any{"error": cliMsgGenerationStopped})
	r.mutateSession(func(s *chatstore.Session) {
		for j := start; j < len(invs); j++ {
			var tm chatstore.Message
			if id := toolIDs[j]; id != "" {
				tm = chatstore.Message{Role: "tool", ToolCallID: id, Content: payload}
			} else {
				tm = chatstore.Message{Role: "user", Content: "tool_result(" + payload + ")"}
			}
			checkpoint.StampMsg(&tm, s, astSeq)
			s.Messages = append(s.Messages, tm)
			s.LastMessageAt = time.Now()
		}
	})
	return r.persistSession()
}

func (r *Runtime) printToolLine(cpSeq int, branchKey, name string, rawArgs json.RawMessage) {
	prefix := checkpoint.FormatLinePrefix(cpSeq, branchKey)
	for _, line := range formatToolDisplayLines(name, rawArgs) {
		fmt.Fprintf(r.Out, "%s%s\n", prefix, line)
	}
}

func toolingResultJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"error":"marshal"}`
	}
	return string(b)
}

func (r *Runtime) streamOptsWithRetry(showThinking bool, reasonSink io.Writer) llm.StreamOpts {
	opts := llm.StreamOpts{ShowThinking: showThinking, ReasoningSink: reasonSink}
	r.bindAPIRetry(&opts)
	return opts
}

func (r *Runtime) bindAPIRetry(opts *llm.StreamOpts) {
	if opts == nil {
		return
	}
	out := r.Out
	opts.OnRetry = func(attempt, max int, err error, wait time.Duration) {
		line := llm.RetryMessage(attempt, max, err, wait)
		if out != nil && !r.machineMode() {
			fmt.Fprintf(out, "\n%s\n", termcolor.WrapRed(line))
			flushWriter(out)
			return
		}
		logging.Log(logging.WARNING_LOG_LEVEL, line, logging.LogOptions{Params: map[string]any{
			"attempt": attempt,
			"max":     max,
			"wait_ms": wait.Milliseconds(),
		}})
	}
}
