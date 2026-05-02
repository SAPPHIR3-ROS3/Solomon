package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
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
)

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
	return r.onUserMessage(ctx, strings.TrimSpace(line))
}

func (r *Runtime) persistSession() error {
	if r.EphemeralSession {
		return nil
	}
	return chatstore.WriteSession(r.ProjHex, r.Session)
}

func (r *Runtime) Run(ctx context.Context) error {
	logging.Log(logging.INFO_LOG_LEVEL, "interactive REPL started")
	printWelcomeBanner(r.Out, r.Cfg, r.Model, r.ProjHex, r.ProjRoot)
	for {
		line, err := r.RL.Readline()
		if err != nil {
			switch {
			case errors.Is(err, io.EOF):
				logging.Log(logging.INFO_LOG_LEVEL, "interactive session ended (EOF)")
			case errors.Is(err, readline.ErrInterrupt):
				logging.Log(logging.WARNING_LOG_LEVEL, "interactive session interrupted")
			default:
				logging.Log(logging.ERROR_LOG_LEVEL, "readline failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			}
			return err
		}
		line = strings.TrimSpace(line)
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
		if err := r.onUserMessage(ctx, line); err != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, "onUserMessage failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			fmt.Fprintf(r.Out, "error: %v\n", err)
		}
	}
}

func (r *Runtime) onUserMessage(ctx context.Context, line string) error {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "!") {
		cmd := strings.TrimSpace(strings.TrimPrefix(line, "!"))
		if cmd == "" {
			return nil
		}
		return r.runUserShellLine(ctx, cmd)
	}
	if !r.EphemeralSession && r.Session.ID == "" && len(r.Session.Messages) == 0 {
		r.Session.ID = chatstore.NewPlaceholderChatID(time.Now())
	}
	if r.EphemeralSession && r.Session.Title == "" && len(r.Session.Messages) == 0 {
		t, err := title.FromPrompt(ctx, r.Client, r.Cfg, r.Model, line)
		if err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "ephemeral title from model failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		}
		if err != nil || strings.TrimSpace(t) == "" {
			t = title.FallbackFromWords(line)
		}
		t = title.NormalizeSlug(t)
		r.Session.Title = t
		r.Session.ID = chatstore.ChatIDHex(t, r.Session.CreatedAt)
	}
	r.Session.Messages = append(r.Session.Messages, chatstore.Message{Role: "user", Content: line})
	r.Session.LastMessageAt = time.Now()
	r.Session.LastUserMessageAt = time.Now()
	if err := r.persistSession(); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "persist session failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return err
	}
	if err := r.runAgentTurns(ctx); err != nil {
		return err
	}
	if !r.EphemeralSession && chatstore.IsPlaceholderChatID(r.Session.ID) {
		return r.finalizeDeferredChatTitle(ctx)
	}
	return nil
}

func (r *Runtime) finalizeDeferredChatTitle(ctx context.Context) error {
	var firstUser string
	for _, m := range r.Session.Messages {
		if m.Role == "user" && strings.TrimSpace(m.Content) != "" && !strings.HasPrefix(m.Content, "tool_result(") {
			firstUser = m.Content
			break
		}
	}
	if firstUser == "" {
		return nil
	}
	t, err := title.FromPrompt(ctx, r.Client, r.Cfg, r.Model, firstUser)
	if err != nil || strings.TrimSpace(t) == "" {
		t = title.FallbackFromWords(firstUser)
	}
	t = title.NormalizeSlug(t)
	oldID := r.Session.ID
	r.Session.Title = t
	r.Session.ID = chatstore.ChatIDHex(t, r.Session.CreatedAt)
	if err := chatstore.RenameSessionFile(r.ProjHex, oldID, r.Session.ID); err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "rename session file failed", logging.LogOptions{Params: map[string]any{"old_id": oldID, "new_id": r.Session.ID, "err": err.Error()}})
		if err := r.persistSession(); err != nil {
			return err
		}
		_ = chatstore.RemoveSessionPath(r.ProjHex, oldID)
		return nil
	}
	return r.persistSession()
}

func (r *Runtime) runAgentTurns(ctx context.Context) error {
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
		fmt.Fprintf(r.Out, "%s ", termcolor.WrapAssistant(r.Model+":"))
		turn, err := llm.StreamAssistantTurn(ctx, r.Client, params, termcolor.NewToolLineWriter(r.Out), llm.StreamOpts{ShowThinking: r.Cfg.ShowThinking, ReasoningSink: r.Out})
		fmt.Fprintln(r.Out)
		if err != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, "assistant stream failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			return err
		}
		if r.Cfg.UsageStatsEnabled() {
			ctxTok, usrTok, ctxEst := llm.UsagePromptParts(sys, msgs, turn.Usage.PromptTokens, turn.Usage.CachedPromptTokens)
			fmt.Fprintln(r.Out, termcolor.UsageTokensLine(ctxTok, usrTok, turn.Usage.ReasoningTokens, turn.Usage.ResponseTokens, turn.Usage.TotalTokens, turn.Usage.OutputTPS, turn.Usage.TTFTSecs, turn.Usage.PromptTPS, ctxEst))
		}
		ast := chatstore.Message{Role: "assistant", Content: turn.Content}
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
				deps := r.slashDeps(ctx)
				if r.EphemeralSession {
					body, err := commands.SummarizeBody(deps)
					if err != nil {
						logging.Log(logging.WARNING_LOG_LEVEL, "ephemeral auto-summarize failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
						fmt.Fprintf(r.Out, "auto-compact: %v\n", err)
						return nil
					}
					r.Session.Messages = []chatstore.Message{{Role: "assistant", Content: body}}
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
		for i, inv := range invs {
			r.printToolLine(inv.Name, inv.Args)
			res, err := r.execTool(ctx, inv)
			if err != nil {
				logging.Log(logging.WARNING_LOG_LEVEL, "tool execution failed", logging.LogOptions{Params: map[string]any{"tool": inv.Name, "err": err.Error()}})
				res = map[string]any{"error": err.Error()}
			}
			payload := toolingResultJSON(res)
			if id := toolIDs[i]; id != "" {
				r.Session.Messages = append(r.Session.Messages, chatstore.Message{Role: "tool", ToolCallID: id, Content: payload})
			} else {
				r.Session.Messages = append(r.Session.Messages, chatstore.Message{Role: "user", Content: "tool_result(" + payload + ")"})
			}
			r.Session.LastMessageAt = time.Now()
			_ = r.persistSession()
		}
	}
}

func (r *Runtime) printToolLine(name string, rawArgs json.RawMessage) {
	s := string(rawArgs)
	if len(rawArgs) > 0 && json.Valid(rawArgs) {
		var buf bytes.Buffer
		if err := json.Compact(&buf, rawArgs); err == nil {
			s = buf.String()
		}
	}
	fmt.Fprintf(r.Out, "%s\n", termcolor.WrapTool(fmt.Sprintf("Tool: %s(%s)", name, s)))
}

func toolingResultJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"error":"marshal"}`
	}
	return string(b)
}
