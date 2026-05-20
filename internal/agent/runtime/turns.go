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

	"github.com/SAPPHIR3-ROS3/Solomon/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/title"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/param"
	"github.com/openai/openai-go/v2/shared"
)

func flushWriter(w io.Writer) {
	if f, ok := w.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
}

func showGenerationStopped(out io.Writer) {
	fmt.Fprintf(out, "%s\n", termcolor.WrapRed("["+cliMsgGenerationStopped+"]"))
	flushWriter(out)
}

func (r *Runtime) onUserMessage(ctx context.Context, line string, fromReadline bool) error {
	clean, _ := parseMultilineControlRunes(line)
	line = trimMessageEdges(clean)
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
		um = chatstore.Message{Role: "user", Content: line}
		checkpoint.StampMsg(&um, s, seq)
		s.Messages = append(s.Messages, um)
		chatstore.RepairSessionMalformedImages(s)
		s.LastMessageAt = time.Now()
		s.LastUserMessageAt = time.Now()
	})
	if firstUserLine != "" {
		go r.refineEphemeralTitle(ctx, firstUserLine)
	}
	if !fromReadline {
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
		if !r.Cfg.UsageStatsEnabled() || len(usageTurns) == 0 {
			usageTurns = nil
			return
		}
		agg := llm.AggregateConsecutiveTurnUsage(usageTurns)
		ctxTok, usrTok, ctxEst, reasonTok, respTok, totalTok := llm.UsageTokensDisplayParts(usageSys, usageMsgs, agg, len(usageTurns))
		fmt.Fprintln(r.Out, termcolor.UsageTokensLine(ctxTok, usrTok, reasonTok, respTok, totalTok, agg.OutputTPS, agg.TTFTSecs, agg.PromptTPS, ctxEst, agg.TurnWallSecs))
		usageTurns = nil
	}
	for {
		sys, err := r.systemPrompt(r.Cfg.ReasoningEffortIsNone())
		if err != nil {
			return err
		}
		tools, err := r.toolParams()
		if err != nil {
			return err
		}
		msgs, imageFiles := r.sessionMessagesSnapshot()
		params := openai.ChatCompletionNewParams{
			Model:             shared.ChatModel(r.Model),
			Messages:          llm.MessageParams(sys, msgs, imageFiles),
			Tools:             tools,
			ParallelToolCalls: param.NewOpt(true),
		}
		llm.ApplyChatReasoning(r.Cfg, &params, false)
		llm.ApplyMaxResponseTokens(r.Cfg, &params)
		var astSeq int
		var branchKey string
		r.mutateSession(func(s *chatstore.Session) {
			astSeq = checkpoint.Bump(s)
			branchKey = s.CheckpointBranchSuffix
		})
		reasoningEff := "none"
		if lbl := r.Cfg.ReasoningEffortLabel(); lbl != "" {
			reasoningEff = lbl
		}
		fmt.Fprintf(r.Out, "%s%s (%s): ", checkpoint.FormatLinePrefix(astSeq, branchKey), termcolor.WrapAssistant(r.Model), termcolor.WrapThinking(reasoningEff))
		turn, err := llm.StreamAssistantTurn(runCtx, r.Client, params, termcolor.NewToolLineWriter(r.Out), llm.StreamOpts{ShowThinking: r.Cfg.ShowThinking, ReasoningSink: r.Out})
		fmt.Fprintln(r.Out)
		if err != nil {
			if interruptedDuringGeneration(ctx, runCtx, err) {
				flushUsageStats()
				logging.Log(logging.INFO_LOG_LEVEL, cliMsgGenerationStopped)
				showGenerationStopped(r.Out)
				return nil
			}
			flushUsageStats()
			logging.Log(logging.ERROR_LOG_LEVEL, "assistant stream failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			return err
		}
		if r.Cfg.UsageStatsEnabled() {
			usageTurns = append(usageTurns, turn.Usage)
			usageSys, usageMsgs = sys, msgs
		}
		ast := chatstore.Message{Role: "assistant", Content: turn.Content, ReasoningText: strings.TrimSpace(turn.ReasoningText)}
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
		if len(turn.ToolCalls) > 0 {
			for _, tc := range turn.ToolCalls {
				invs = append(invs, tooling.Invocation{Name: tc.Name, Args: json.RawMessage(tc.Arguments)})
				toolIDs = append(toolIDs, tc.ID)
			}
		} else {
			var legacyTools bool
			r.mutateSession(func(s *chatstore.Session) {
				legacyTools = s.LegacyTools
			})
			if legacyTools {
				for _, inv := range tooling.ExtractToolInvocations(turn.Content) {
					invs = append(invs, inv)
					toolIDs = append(toolIDs, "")
				}
			}
		}
		if len(invs) == 0 {
			flushUsageStats()
			if turn.Usage.PromptTokens > 0 && turn.Usage.PromptTokens >= r.CompactionThresholdTokens {
				deps := r.slashDeps(runCtx)
				if r.EphemeralSession {
					body, err := commands.SummarizeBody(deps)
					if err != nil {
						logging.Log(logging.WARNING_LOG_LEVEL, "ephemeral auto-summarize failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
						fmt.Fprintf(r.Out, "auto-compact: %v\n", err)
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
					fmt.Fprintln(r.Out, "context summarized")
					continue
				}
				if err := commands.Summarize(deps); err != nil {
					logging.Log(logging.WARNING_LOG_LEVEL, "auto-compact failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
					fmt.Fprintf(r.Out, "auto-compact: %v\n", err)
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
			r.printToolLine(astSeq, branchKey, inv.Name, inv.Args)
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
			payload := toolingResultJSON(res)
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
