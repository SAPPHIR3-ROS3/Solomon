package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"solomon/internal/chatstore"
	"solomon/internal/config"
	"solomon/internal/llm"
	"solomon/internal/logging"
	"solomon/internal/prompt"
	"solomon/internal/termcolor"
	"solomon/internal/title"
	"solomon/internal/tooling"

	"github.com/chzyer/readline"
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

	Out io.Writer
}

func NewRuntime(rl *readline.Instance, cfg *config.Root, prov *config.Provider, projHex, projRoot string, sess *chatstore.Session) *Runtime {
	cl := openai.NewClient(
		option.WithAPIKey(prov.APIKey),
		option.WithBaseURL(prov.BaseURL),
	)
	return &Runtime{
		RL:       rl,
		Client:   cl,
		Model:    cfg.Current.Model,
		Cfg:      cfg,
		Prov:     prov,
		ProjHex:  projHex,
		ProjRoot: projRoot,
		Mode:     "build",
		Session:  sess,
		Out:      os.Stdout,
	}
}

func (r *Runtime) ApplyCurrentModel(providerName, modelID string) error {
	r.Cfg.Current.Provider = providerName
	r.Cfg.Current.Model = modelID
	if err := config.Save(r.Cfg); err != nil {
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
	d := prompt.Data{
		Tools:      dump,
		Syntax:     prompt.ToolInvocationSyntax(),
		ExtraRules: "",
		Language:   r.Cfg.EffectiveResponseLanguage(),
	}
	if r.Mode == "plan" {
		return prompt.RenderPlan(d)
	}
	return prompt.RenderBuild(d)
}

func (r *Runtime) Run(ctx context.Context) error {
	for {
		line, err := r.RL.Readline()
		if err != nil {
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "/") {
			if err := r.handleSlash(ctx, line); err != nil {
				if errors.Is(err, ErrExitChat) {
					return nil
				}
				fmt.Fprintf(r.Out, "%v\n", err)
			}
			continue
		}
		if err := r.onUserMessage(ctx, line); err != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, err.Error())
			fmt.Fprintf(r.Out, "error: %v\n", err)
		}
	}
}

func (r *Runtime) onUserMessage(ctx context.Context, line string) error {
	if r.Session.Title == "" && len(r.Session.Messages) == 0 {
		t, err := title.FromPrompt(ctx, r.Client, r.Model, line)
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
	if err := chatstore.WriteSession(r.ProjHex, r.Session); err != nil {
		return err
	}
	return r.runAgentTurns(ctx)
}

func (r *Runtime) runAgentTurns(ctx context.Context) error {
	for {
		sys, err := r.systemPrompt()
		if err != nil {
			return err
		}
		tools, err := NativeToolParams(r.Mode)
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
		fmt.Fprintf(r.Out, "%s%s:%s ", termcolor.Assistant, r.Model, termcolor.Reset)
		turn, err := llm.StreamAssistantTurn(ctx, r.Client, params, termcolor.NewToolLineWriter(r.Out), llm.StreamOpts{ShowThinking: r.Cfg.ShowThinking, ReasoningSink: r.Out})
		fmt.Fprintln(r.Out)
		if err != nil {
			return err
		}
		if r.Cfg.UsageStatsEnabled() {
			fmt.Fprintln(r.Out, termcolor.UsageTokensLine(turn.Usage.PromptTokens, turn.Usage.ReasoningTokens, turn.Usage.ResponseTokens, turn.Usage.TotalTokens, turn.Usage.OutputTPS, turn.Usage.TTFTSecs, turn.Usage.PromptTPS))
		}
		ast := chatstore.Message{Role: "assistant", Content: turn.Content}
		for _, tc := range turn.ToolCalls {
			ast.ToolCalls = append(ast.ToolCalls, chatstore.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments})
		}
		r.Session.Messages = append(r.Session.Messages, ast)
		r.Session.LastMessageAt = time.Now()
		_ = chatstore.WriteSession(r.ProjHex, r.Session)
		var invs []tooling.Invocation
		var toolIDs []string
		if len(turn.ToolCalls) > 0 {
			for _, tc := range turn.ToolCalls {
				invs = append(invs, tooling.Invocation{Name: tc.Name, Args: json.RawMessage(tc.Arguments)})
				toolIDs = append(toolIDs, tc.ID)
			}
		} else {
			for _, inv := range tooling.ExtractToolInvocations(turn.Content) {
				invs = append(invs, inv)
				toolIDs = append(toolIDs, "")
			}
		}
		if len(invs) == 0 {
			return nil
		}
		for i, inv := range invs {
			r.printToolLine(inv.Name, inv.Args)
			res, err := r.execTool(ctx, inv)
			if err != nil {
				res = map[string]any{"error": err.Error()}
			}
			payload := toolingResultJSON(res)
			if id := toolIDs[i]; id != "" {
				r.Session.Messages = append(r.Session.Messages, chatstore.Message{Role: "tool", ToolCallID: id, Content: payload})
			} else {
				r.Session.Messages = append(r.Session.Messages, chatstore.Message{Role: "user", Content: "tool_result(" + payload + ")"})
			}
			r.Session.LastMessageAt = time.Now()
			_ = chatstore.WriteSession(r.ProjHex, r.Session)
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
	fmt.Fprintf(r.Out, "%sTool: %s(%s)\n%s", termcolor.Tool, name, s, termcolor.Reset)
}

func toolingResultJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"error":"marshal"}`
	}
	return string(b)
}
