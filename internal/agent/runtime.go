package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/internal/mcp"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/prompt"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/title"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"

	readline "github.com/chzyer/readline"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/openai/openai-go/v2/packages/param"
	"github.com/openai/openai-go/v2/shared"
	"sync"
)

var errUserStopGeneration = errors.New("user stopped generation")

const cliMsgGenerationStopped = "Generation stopped"

func flushWriter(w io.Writer) {
	if f, ok := w.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
}

func showGenerationStopped(out io.Writer) {
	fmt.Fprintf(out, "%s\n", termcolor.WrapRed("["+cliMsgGenerationStopped+"]"))
	flushWriter(out)
}

type Runtime struct {
	RL *readline.Instance

	Client openai.Client
	Model  string
	Cfg    *config.Root
	Prov   *config.Provider

	ProjHex  string
	ProjRoot string

	Mode string

	Session *chatstore.Session

	CompactionThresholdTokens int64

	EphemeralSession bool

	chatPersistMu              sync.Mutex
	deferredTitleScheduleMu    sync.Mutex
	deferredTitleWorkerRunning bool

	Out io.Writer

	MCP *solomonmcp.Manager
}

func NewRuntime(rl *readline.Instance, cfg *config.Root, prov *config.Provider, projHex, projRoot string, sess *chatstore.Session) *Runtime {
	cl := openai.NewClient(
		option.WithAPIKey(prov.APIKey),
		option.WithBaseURL(prov.BaseURL),
	)
	return &Runtime{
		RL:                        rl,
		Client:                    cl,
		Model:                     cfg.Current.Model,
		Cfg:                       cfg,
		Prov:                      prov,
		ProjHex:                   projHex,
		ProjRoot:                  projRoot,
		Mode:                      "build",
		Session:                   sess,
		CompactionThresholdTokens: config.EffectiveCompactionThresholdTokens(cfg),
		Out:                       os.Stdout,
	}
}

func (r *Runtime) ApplyCurrentModel(providerName, modelID string) error {
	r.Cfg.Current.Provider = providerName
	r.Cfg.Current.Model = modelID
	if err := config.Save(r.Cfg); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "save config failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return err
	}
	for i := range r.Cfg.Providers {
		if r.Cfg.Providers[i].Name == providerName {
			p := &r.Cfg.Providers[i]
			r.Prov = p
			r.Model = modelID
			r.Client = openai.NewClient(
				option.WithAPIKey(p.APIKey),
				option.WithBaseURL(p.BaseURL),
			)
			return nil
		}
	}
	return fmt.Errorf("provider %q not found", providerName)
}

func (r *Runtime) refreshReadlinePrompt() {
	if r.RL == nil {
		return
	}
	r.RL.SetPrompt(checkpoint.FormatReplPromptPrefix(r.Session) + termcolor.WrapUser("You: "))
}

func (r *Runtime) refreshReadlinePromptContinue() {
	if r.RL == nil {
		return
	}
	r.RL.SetPrompt(checkpoint.FormatReplPromptPrefix(r.Session) + termcolor.WrapUser("... "))
}

func (r *Runtime) systemPrompt() (string, error) {
	var dump string
	var err error
	if r.Mode == "plan" {
		dump, err = buildPlanToolDump()
	} else {
		dump, err = buildBuildToolDump()
	}
	if err != nil {
		return "", err
	}
	if r.MCP != nil {
		if mcpDump := strings.TrimSpace(r.MCP.ToolDump()); mcpDump != "" {
			dump = strings.TrimSpace(dump + "\n---\n" + mcpDump)
		}
	}
	absWorkspace := r.ProjRoot
	if p, err := filepath.Abs(r.ProjRoot); err == nil {
		absWorkspace = p
	}
	syntax := prompt.NativeToolInvocationSyntax()
	if r.Session.LegacyTools {
		syntax = strings.TrimSpace(syntax + "\n\n" + prompt.LegacyToolInvocationSyntaxAppend())
	}
	d := prompt.Data{
		Tools:                 dump,
		Syntax:                syntax,
		ExtraRules:            "",
		Language:              r.Cfg.EffectiveResponseLanguage(),
		UserName:              strings.TrimSpace(r.Cfg.UserName),
		WorkspaceAbsolutePath: absWorkspace,
	}
	if r.Mode == "plan" {
		return prompt.RenderPlan(d)
	}
	return prompt.RenderBuild(d)
}

func (r *Runtime) RunPromptOnce(ctx context.Context, line string) error {
	clean, _ := stripSoftNewlineMarker(line)
	return r.onUserMessage(ctx, trimMessageEdges(clean), false)
}

func (r *Runtime) persistSession() error {
	if r.EphemeralSession {
		return nil
	}
	r.chatPersistMu.Lock()
	defer r.chatPersistMu.Unlock()
	return chatstore.WriteSession(r.ProjHex, r.Session)
}

func (r *Runtime) persistSessionUnsafe() error {
	return chatstore.WriteSession(r.ProjHex, r.Session)
}

func (r *Runtime) Run(ctx context.Context) error {
	logging.Log(logging.INFO_LOG_LEVEL, "interactive REPL started")
	chatstore.FinishSessionLoad(r.Session)
	printWelcomeBanner(r.Out, r.Cfg, r.Model, r.ProjHex, r.ProjRoot)
	var pendingMultiline []string
	for {
		chatstore.FinishSessionLoad(r.Session)
		if len(pendingMultiline) > 0 {
			r.refreshReadlinePromptContinue()
		} else {
			r.refreshReadlinePrompt()
		}
		line, err := r.RL.Readline()
		if err != nil {
			switch {
			case errors.Is(err, io.EOF):
				logging.Log(logging.INFO_LOG_LEVEL, "interactive session ended (EOF)")
			case errors.Is(err, readline.ErrInterrupt):
				logging.Log(logging.INFO_LOG_LEVEL, "interactive session ended (Ctrl+C at prompt)")
				commands.ExitMessage(r.slashDeps(ctx))
				return nil
			default:
				logging.Log(logging.ERROR_LOG_LEVEL, "readline failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			}
			return err
		}
		line, isSoftBreak := stripSoftNewlineMarker(line)
		if isSoftBreak {
			pendingMultiline = append(pendingMultiline, line)
			continue
		}
		if len(pendingMultiline) > 0 {
			pendingMultiline = append(pendingMultiline, line)
			line = strings.Join(pendingMultiline, "\n")
			pendingMultiline = nil
		}
		line = trimMessageEdges(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "/") {
			if err := r.handleSlash(ctx, line); err != nil {
				if errors.Is(err, ErrExitChat) {
					logging.Log(logging.INFO_LOG_LEVEL, "user requested exit from chat")
					return nil
				}
				logging.Log(logging.WARNING_LOG_LEVEL, "slash command failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
				fmt.Fprintf(r.Out, "%v\n", err)
			}
			continue
		}
		if err := r.onUserMessage(ctx, line, true); err != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, "onUserMessage failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			fmt.Fprintf(r.Out, "error: %v\n", err)
		}
	}
}

func (r *Runtime) onUserMessage(ctx context.Context, line string, fromReadline bool) error {
	clean, _ := stripSoftNewlineMarker(line)
	line = trimMessageEdges(clean)
	if strings.HasPrefix(line, "!") {
		cmd := trimMessageEdges(strings.TrimPrefix(line, "!"))
		if cmd == "" {
			return nil
		}
		return r.runUserShellLine(ctx, cmd)
	}
	if !r.EphemeralSession && r.Session.ID == "" && len(r.Session.Messages) == 0 {
		r.Session.ID = chatstore.NewPlaceholderChatID(time.Now())
	}
	if r.EphemeralSession && r.Session.Title == "" && len(r.Session.Messages) == 0 {
		tSlug := title.NormalizeSlug(title.FallbackFromWords(line))
		r.Session.Title = tSlug
		r.Session.ID = chatstore.ChatIDHex(tSlug, r.Session.CreatedAt)
		go r.refineEphemeralTitle(ctx, strings.TrimSpace(line))
	}
	seq := checkpoint.Bump(r.Session)
	um := chatstore.Message{Role: "user", Content: line}
	checkpoint.StampMsg(&um, r.Session, seq)
	r.Session.Messages = append(r.Session.Messages, um)
	r.Session.LastMessageAt = time.Now()
	r.Session.LastUserMessageAt = time.Now()
	if !fromReadline {
		fmt.Fprintf(r.Out, "%s%s %s\n", checkpoint.FormatLinePrefix(um.CheckpointSeq, um.CheckpointBranchKey), termcolor.WrapUser("You:"), line)
	}
	if err := r.persistSession(); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "persist session failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return err
	}
	if err := r.runAgentTurns(ctx); err != nil {
		return err
	}
	if !r.EphemeralSession && chatstore.IsPlaceholderChatID(r.Session.ID) {
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
	for {
		sys, err := r.systemPrompt()
		if err != nil {
			return err
		}
		tools, err := r.toolParams()
		if err != nil {
			return err
		}
		msgs := r.Session.Messages
		params := openai.ChatCompletionNewParams{
			Model:             shared.ChatModel(r.Model),
			Messages:          llm.MessageParams(sys, msgs),
			ReasoningEffort:   r.Cfg.GlobalReasoningEffort(),
			Tools:             tools,
			ParallelToolCalls: param.NewOpt(true),
		}
		llm.ApplyMaxResponseTokens(r.Cfg, &params)
		astSeq := checkpoint.Bump(r.Session)
		reasoningEff := "none"
		if lbl := r.Cfg.ReasoningEffortLabel(); lbl != "" {
			reasoningEff = lbl
		}
		fmt.Fprintf(r.Out, "%s%s (%s): ", checkpoint.FormatLinePrefix(astSeq, r.Session.CheckpointBranchSuffix), termcolor.WrapAssistant(r.Model), termcolor.WrapThinking(reasoningEff))
		turn, err := llm.StreamAssistantTurn(runCtx, r.Client, params, termcolor.NewToolLineWriter(r.Out), llm.StreamOpts{ShowThinking: r.Cfg.ShowThinking, ReasoningSink: r.Out})
		fmt.Fprintln(r.Out)
		if err != nil {
			if interruptedDuringGeneration(ctx, runCtx, err) {
				logging.Log(logging.INFO_LOG_LEVEL, cliMsgGenerationStopped)
				showGenerationStopped(r.Out)
				return nil
			}
			logging.Log(logging.ERROR_LOG_LEVEL, "assistant stream failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			return err
		}
		if r.Cfg.UsageStatsEnabled() {
			ctxTok, usrTok, ctxEst := llm.UsagePromptParts(sys, msgs, turn.Usage.PromptTokens, turn.Usage.CachedPromptTokens)
			fmt.Fprintln(r.Out, termcolor.UsageTokensLine(ctxTok, usrTok, turn.Usage.ReasoningTokens, turn.Usage.ResponseTokens, turn.Usage.TotalTokens, turn.Usage.OutputTPS, turn.Usage.TTFTSecs, turn.Usage.PromptTPS, ctxEst, turn.Usage.TurnWallSecs))
		}
		ast := chatstore.Message{Role: "assistant", Content: turn.Content, ReasoningText: strings.TrimSpace(turn.ReasoningText)}
		checkpoint.StampMsg(&ast, r.Session, astSeq)
		for _, tc := range turn.ToolCalls {
			ast.ToolCalls = append(ast.ToolCalls, chatstore.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments})
		}
		llm.PopulateAssistantTurnUsage(&ast, sys, msgs, turn.Usage)
		chatstore.BackfillAssistantUsageFromTextIfEmpty(&ast, msgs)
		r.Session.Messages = append(r.Session.Messages, ast)
		r.Session.LastMessageAt = time.Now()
		_ = r.persistSession()
		var invs []tooling.Invocation
		var toolIDs []string
		if len(turn.ToolCalls) > 0 {
			for _, tc := range turn.ToolCalls {
				invs = append(invs, tooling.Invocation{Name: tc.Name, Args: json.RawMessage(tc.Arguments)})
				toolIDs = append(toolIDs, tc.ID)
			}
		} else if r.Session.LegacyTools {
			for _, inv := range tooling.ExtractToolInvocations(turn.Content) {
				invs = append(invs, inv)
				toolIDs = append(toolIDs, "")
			}
		}
		if len(invs) == 0 {
			if turn.Usage.PromptTokens > 0 && turn.Usage.PromptTokens >= r.CompactionThresholdTokens {
				deps := r.slashDeps(runCtx)
				if r.EphemeralSession {
					body, err := commands.SummarizeBody(deps)
					if err != nil {
						logging.Log(logging.WARNING_LOG_LEVEL, "ephemeral auto-summarize failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
						fmt.Fprintf(r.Out, "auto-compact: %v\n", err)
						return nil
					}
					r.Session.Messages = []chatstore.Message{{Role: "assistant", Content: body}}
					r.Session.MainOrphans = nil
					r.Session.CheckpointBranchSuffix = ""
					r.Session.ForkChildCount = nil
					r.Session.CheckpointLast = -1
					r.Session.LastCommitOID = ""
					r.Session.LastMessageAt = time.Now()
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
				showGenerationStopped(r.Out)
				return nil
			}
			inv := invs[i]
			r.printToolLine(astSeq, r.Session.CheckpointBranchSuffix, inv.Name, inv.Args)
			res, err := r.execTool(runCtx, inv)
			if interruptedDuringGeneration(ctx, runCtx, err) {
				if err2 := r.appendSyntheticToolResults(astSeq, invs, toolIDs, i); err2 != nil {
					return err2
				}
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
			checkpoint.StampMsg(&tm, r.Session, astSeq)
			r.Session.Messages = append(r.Session.Messages, tm)
			r.Session.LastMessageAt = time.Now()
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
	for j := start; j < len(invs); j++ {
		var tm chatstore.Message
		if id := toolIDs[j]; id != "" {
			tm = chatstore.Message{Role: "tool", ToolCallID: id, Content: payload}
		} else {
			tm = chatstore.Message{Role: "user", Content: "tool_result(" + payload + ")"}
		}
		checkpoint.StampMsg(&tm, r.Session, astSeq)
		r.Session.Messages = append(r.Session.Messages, tm)
		r.Session.LastMessageAt = time.Now()
	}
	return r.persistSession()
}

func (r *Runtime) printToolLine(cpSeq int, branchKey, name string, rawArgs json.RawMessage) {
	s := string(rawArgs)
	if len(rawArgs) > 0 && json.Valid(rawArgs) {
		var buf bytes.Buffer
		if err := json.Compact(&buf, rawArgs); err == nil {
			s = buf.String()
		}
	}
	fmt.Fprintf(r.Out, "%s%s\n", checkpoint.FormatLinePrefix(cpSeq, branchKey), termcolor.WrapTool(fmt.Sprintf("Tool: %s(%s)", name, s)))
}

func toolingResultJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"error":"marshal"}`
	}
	return string(b)
}
