package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"solomon/internal/chatstore"
	"solomon/internal/config"
	"solomon/internal/llm"
	"solomon/internal/logging"
	"solomon/internal/modelsapi"
	"solomon/internal/termcolor"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/shared"
)

var ErrExitChat = errors.New("exit chat")

func (r *Runtime) handleSlash(ctx context.Context, line string) error {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}
	name := strings.TrimPrefix(parts[0], "/")
	switch name {
	case "plan":
		r.Mode = "plan"
		fmt.Fprintln(r.Out, "Mode: plan")
		return nil
	case "build":
		r.Mode = "build"
		fmt.Fprintln(r.Out, "Mode: build")
		return nil
	case "clear":
		fmt.Fprint(r.Out, "\033[2J\033[H")
		return nil
	case "log":
		if len(parts) < 2 {
			return fmt.Errorf("usage: /log {error|warning|info|debug|result}")
		}
		lvl, err := logging.ParseLevel(parts[1])
		if err != nil {
			return err
		}
		if err := logging.SetGlobalLevel(lvl); err != nil {
			return err
		}
		fmt.Fprintf(r.Out, "Log level: %s\n", logging.LevelLabel(lvl))
		return nil
	case "reasoning":
		if len(parts) < 2 {
			if lab := r.Cfg.ReasoningEffortLabel(); lab != "" {
				fmt.Fprintf(r.Out, "reasoning_effort=%s (main chat only; subagent omits reasoning)\n", lab)
			} else {
				fmt.Fprintln(r.Out, "reasoning_effort unset (provider default); subagent never sends reasoning_effort")
			}
			return nil
		}
		canonical, err := config.ParseReasoningEffortToken(parts[1])
		if err != nil {
			return err
		}
		r.Cfg.ReasoningEffort = canonical
		if err := config.Save(r.Cfg); err != nil {
			return err
		}
		fmt.Fprintf(r.Out, "reasoning_effort=%s (saved; main chat only)\n", canonical)
		return nil
	case "timeout":
		if len(parts) < 2 {
			return fmt.Errorf("usage: /timeout <minutes>")
		}
		n, err := strconv.Atoi(parts[1])
		if err != nil {
			return err
		}
		if err := config.ClampTimeoutMinutes(n); err != nil {
			return err
		}
		r.Cfg.SubagentTimeoutMinutes = n
		if err := config.Save(r.Cfg); err != nil {
			return err
		}
		fmt.Fprintf(r.Out, "subagent_timeout_minutes=%d\n", n)
		return nil
	case "stats":
		next := !r.Cfg.UsageStatsEnabled()
		r.Cfg.ShowUsageStats = &next
		if err := config.Save(r.Cfg); err != nil {
			return err
		}
		onOff := "off"
		if next {
			onOff = "on"
		}
		fmt.Fprintf(r.Out, "token stats: %s\n", onOff)
		return nil
	case "thinking":
		if len(parts) < 2 {
			r.Cfg.ShowThinking = !r.Cfg.ShowThinking
			if err := config.Save(r.Cfg); err != nil {
				return err
			}
			onOff := "off"
			if r.Cfg.ShowThinking {
				onOff = "on"
			}
			fmt.Fprintf(r.Out, "streaming reasoning preview: %s\n", onOff)
			return nil
		}
		sw := strings.ToLower(parts[1])
		switch sw {
		case "on", "yes", "true", "show", "1":
			r.Cfg.ShowThinking = true
		case "off", "no", "false", "hide", "0":
			r.Cfg.ShowThinking = false
		default:
			return fmt.Errorf("usage: /thinking | /thinking on|off")
		}
		if err := config.Save(r.Cfg); err != nil {
			return err
		}
		onOff := "off"
		if r.Cfg.ShowThinking {
			onOff = "on"
		}
		fmt.Fprintf(r.Out, "streaming reasoning preview: %s\n", onOff)
		return nil
	case "max_response":
		if len(parts) < 2 {
			if r.Cfg.MaxResponseTokens > 0 {
				fmt.Fprintf(r.Out, "max_response_tokens=%d (max_completion_tokens)\n", r.Cfg.MaxResponseTokens)
			} else {
				fmt.Fprintln(r.Out, "max_response_tokens unset (provider/model default)")
			}
			return nil
		}
		n, err := strconv.Atoi(parts[1])
		if err != nil {
			return err
		}
		if n < 1 {
			return fmt.Errorf("max_response must be >= 1")
		}
		r.Cfg.MaxResponseTokens = n
		if err := config.Save(r.Cfg); err != nil {
			return err
		}
		fmt.Fprintf(r.Out, "max_response_tokens=%d\n", n)
		return nil
	case "models":
		return r.slashModels(ctx)
	case "connect":
		return r.slashConnect(ctx)
	case "resume":
		return r.slashResume(ctx, parts[1:])
	case "summarize", "compact":
		return r.slashSummarize(ctx)
	case "exit", "quit":
		r.slashExitQuit()
		return ErrExitChat
	case "language":
		if len(parts) < 2 {
			stored := strings.TrimSpace(r.Cfg.ResponseLanguage)
			eff := r.Cfg.EffectiveResponseLanguage()
			if stored == "" {
				fmt.Fprintf(r.Out, "response_language=%s (default)\n", eff)
			} else {
				fmt.Fprintf(r.Out, "response_language=%s\n", eff)
			}
			return nil
		}
		rest := strings.Join(parts[1:], " ")
		rest = strings.TrimSpace(rest)
		if rest == "" {
			return fmt.Errorf("usage: /language | /language <language> | /language clear")
		}
		switch strings.ToLower(rest) {
		case "clear", "default", "reset":
			r.Cfg.ResponseLanguage = ""
		default:
			r.Cfg.ResponseLanguage = rest
		}
		if err := config.Save(r.Cfg); err != nil {
			return err
		}
		if strings.TrimSpace(r.Cfg.ResponseLanguage) != "" {
			fmt.Fprintf(r.Out, "response_language=%s (saved; injected into system prompt)\n", strings.TrimSpace(r.Cfg.ResponseLanguage))
		} else {
			fmt.Fprintf(r.Out, "response_language reset to default %s (saved)\n", config.DefaultResponseLanguage)
		}
		return nil
	case "help":
		writeSlashHelp(r.Out)
		return nil
	default:
		return fmt.Errorf("unknown command /%s (try /help)", name)
	}
}

func slashRegistry() [][]string {
	return [][]string{
		{"/plan", "planning tools only"},
		{"/build", "build tools (shell, files, subagent)"},
		{"/clear", "clear terminal (ANSI)"},
		{"/connect", "add OpenAI-compatible provider"},
		{"/exit, /quit", "exit and show how to resume"},
		{"/help", "this list"},
		{"/language", "/language | /language <language> | /language clear — reply language (default English; saved; system prompt)"},
		{"/log", "/log {error|warning|info|debug|result} visible log verbosity"},
		{"/max_response", "/max_response | /max_response <n> assistant output cap (tokens, n>=1)"},
		{"/models", "list models and switch current model"},
		{"/reasoning", "/reasoning | /reasoning {none|low|med|high} main chat reasoning_effort"},
		{"/resume", "/resume | /resume <id|title>"},
		{"/stats", "toggle token usage line after assistant turns (saved)"},
		{"/summarize, /compact", "summarize full chat; summary + last 8 msgs; then /clear"},
		{"/thinking", "/thinking toggles preview; /thinking on|off streamed model reasoning (light yellow)"},
		{"/timeout", "/timeout <minutes> subagent segment (1–180)"},
	}
}

func writeSlashHelp(w io.Writer) {
	rows := slashRegistry()
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	maxCmd := 0
	for _, row := range rows {
		if n := len(row[0]); n > maxCmd {
			maxCmd = n
		}
	}
	for _, row := range rows {
		fmt.Fprintf(w, "%-*s\t%s\n", maxCmd, row[0], row[1])
	}
}

func (r *Runtime) slashExitQuit() {
	fmt.Fprintln(r.Out, "Goodbye.")
	if r.Session != nil && r.Session.ID != "" {
		fmt.Fprintf(r.Out, "Resume this chat by id:   /resume %s\n", r.Session.ID)
		if r.Session.Title != "" {
			fmt.Fprintf(r.Out, "Resume this chat by title: /resume %s\n", r.Session.Title)
		}
		return
	}
	fmt.Fprintln(r.Out, "This chat has no id yet (send a message first). To resume a saved chat:")
	fmt.Fprintln(r.Out, "  /resume              — list recent chats")
	fmt.Fprintln(r.Out, "  /resume <id>         — open by session id")
	fmt.Fprintln(r.Out, "  /resume <title>      — open by exact title")
}

type listedModel struct {
	Prov  string
	Model string
}

func (r *Runtime) slashModels(ctx context.Context) error {
	var rows []listedModel
	for i := range r.Cfg.Providers {
		p := &r.Cfg.Providers[i]
		ids, err := modelsapi.List(p.BaseURL, p.APIKey)
		if err != nil {
			fmt.Fprintf(r.Out, "provider %s: error: %v\n", p.Name, err)
			continue
		}
		for _, mid := range ids {
			rows = append(rows, listedModel{p.Name, mid})
		}
	}
	if len(rows) == 0 {
		return fmt.Errorf("no models available")
	}
	nShow := len(rows)
	if len(rows) > 20 {
		nShow = 20
	}
	for i := 0; i < nShow; i++ {
		mark := ""
		if r.Cfg.Current.Provider == rows[i].Prov && r.Cfg.Current.Model == rows[i].Model {
			mark = "\t(current)"
		}
		fmt.Fprintf(r.Out, "%d\t%s[%s]%s\n", i, rows[i].Model, rows[i].Prov, mark)
	}
	if len(rows) > 20 {
		fmt.Fprintln(r.Out, "...")
	}
	br := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprintf(r.Out, "Select: index 0-%d", nShow-1)
		if len(rows) > 20 {
			fmt.Fprint(r.Out, ", 20 to enter a model id, or paste exact model id")
		} else {
			fmt.Fprint(r.Out, ", or paste exact model id")
		}
		fmt.Fprint(r.Out, "\n> ")
		br.Scan()
		line := strings.TrimSpace(br.Text())
		if line == "" {
			fmt.Fprintln(r.Out, "Invalid: empty input.")
			continue
		}
		if len(rows) > 20 {
			ok, msg := r.trySlashModelPick(rows, line, br)
			if ok {
				fmt.Fprintf(r.Out, "Using %s[%s]\n", r.Model, r.Prov.Name)
				return nil
			}
			fmt.Fprintf(r.Out, "Invalid: %s\n", msg)
			continue
		}
		if config.AllDigits(line) {
			n, err := strconv.Atoi(line)
			if err != nil {
				fmt.Fprintln(r.Out, "Invalid: not a valid number.")
				continue
			}
			if n < 0 || n >= len(rows) {
				fmt.Fprintf(r.Out, "Invalid: index must be between 0 and %d.\n", len(rows)-1)
				continue
			}
			sel := rows[n]
			if err := r.ApplyCurrentModel(sel.Prov, sel.Model); err != nil {
				return err
			}
			fmt.Fprintf(r.Out, "Using %s[%s]\n", r.Model, r.Prov.Name)
			return nil
		}
		if err := r.resolveModelPaste(rows, line); err != nil {
			fmt.Fprintf(r.Out, "Invalid: %v\n", err)
			continue
		}
		fmt.Fprintf(r.Out, "Using %s[%s]\n", r.Model, r.Prov.Name)
		return nil
	}
}

func (r *Runtime) trySlashModelPick(rows []listedModel, line string, br *bufio.Scanner) (ok bool, errMsg string) {
	if config.AllDigits(line) {
		n, err := strconv.Atoi(line)
		if err != nil {
			return false, "not a valid number."
		}
		if n >= 0 && n < 20 {
			sel := rows[n]
			if err := r.ApplyCurrentModel(sel.Prov, sel.Model); err != nil {
				return false, err.Error()
			}
			return true, ""
		}
		if n == 20 {
			for {
				fmt.Fprint(r.Out, "Model id: ")
				br.Scan()
				id := strings.TrimSpace(br.Text())
				if id == "" {
					fmt.Fprintln(r.Out, "Invalid: empty model id.")
					continue
				}
				if err := r.resolveModelPaste(rows, id); err != nil {
					fmt.Fprintf(r.Out, "Invalid: %v\n", err)
					continue
				}
				return true, ""
			}
		}
		return false, "index must be 0-19 or 20 to type an id."
	}
	if err := r.resolveModelPaste(rows, line); err != nil {
		return false, err.Error()
	}
	return true, ""
}

func (r *Runtime) resolveModelPaste(rows []listedModel, id string) error {
	var matches []listedModel
	for _, row := range rows {
		if row.Model == id {
			matches = append(matches, row)
		}
	}
	if len(matches) == 0 {
		return fmt.Errorf("model id %q not in the listed models", id)
	}
	if len(matches) > 1 {
		return fmt.Errorf("model id %q exists for multiple providers; use a numeric index 0-19", id)
	}
	return r.ApplyCurrentModel(matches[0].Prov, matches[0].Model)
}

func (r *Runtime) slashConnect(ctx context.Context) error {
	br := bufio.NewReader(os.Stdin)
	fmt.Fprint(r.Out, "Provider display name: ")
	n, _ := br.ReadString('\n')
	fmt.Fprint(r.Out, "Base URL: ")
	u, _ := br.ReadString('\n')
	fmt.Fprint(r.Out, "API key: ")
	k, _ := br.ReadString('\n')
	base, err := config.NormalizeAPIBase(strings.TrimSpace(u))
	if err != nil {
		return err
	}
	prov := config.Provider{Name: strings.TrimSpace(n), BaseURL: base, APIKey: strings.TrimSpace(k)}
	r.Cfg.Providers = append(r.Cfg.Providers, prov)
	r.Cfg.Current.Provider = prov.Name
	return config.Save(r.Cfg)
}

func (r *Runtime) slashResume(ctx context.Context, args []string) error {
	if len(args) == 0 {
		list, err := chatstore.ListRecent(r.ProjHex, 10)
		if err != nil {
			return err
		}
		for i, s := range list {
			fmt.Fprintf(r.Out, "%d\t%s\t%s\n", i, s.ID, s.Title)
		}
		fmt.Fprint(r.Out, "pick number or /resume <id|title>\n")
		return nil
	}
	arg := strings.TrimSpace(args[0])
	sess, err := chatstore.ReadSession(r.ProjHex, arg)
	if err != nil {
		sess, err = chatstore.FindByTitle(r.ProjHex, arg)
	}
	if err != nil {
		return err
	}
	r.Session = sess
	fmt.Fprintf(r.Out, "loaded chat %s\n", sess.ID)
	return nil
}

func formatChatTranscript(msgs []chatstore.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		switch m.Role {
		case "user":
			fmt.Fprintf(&b, "User:\n%s\n\n", m.Content)
		case "assistant":
			fmt.Fprintf(&b, "Assistant:\n%s\n\n", m.Content)
			if len(m.ToolCalls) > 0 {
				for _, tc := range m.ToolCalls {
					fmt.Fprintf(&b, "  [tool_call %s] %s(%s)\n", tc.ID, tc.Name, tc.Arguments)
				}
				fmt.Fprintf(&b, "\n")
			}
		case "tool":
			fmt.Fprintf(&b, "Tool[%s]:\n%s\n\n", m.ToolCallID, m.Content)
		default:
			fmt.Fprintf(&b, "%s:\n%s\n\n", m.Role, m.Content)
		}
	}
	return b.String()
}

func compactSummaryBody(sep, summaryLLM, retainedBlock string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n[Conversation summary]\n%s\n\n%s\n\n", sep, sep, summaryLLM)
	fmt.Fprintf(&b, "%s\n[Messaggi conservati]\n%s\n\n%s\n\n%s\n", sep, sep, retainedBlock, sep)
	return b.String()
}

func (r *Runtime) slashSummarize(ctx context.Context) error {
	msgs := r.Session.Messages
	if len(msgs) == 0 {
		return fmt.Errorf("no messages to summarize")
	}
	fmt.Fprintln(r.Out, "Riassunto in corso…")
	transcript := formatChatTranscript(msgs)
	sys := `You summarize technical conversations concisely. Preserve important facts: decisions, file paths, commands, errors, and open tasks. Match the language of the transcript. Output only the summary text, without preamble or meta-commentary.`
	params := openai.ChatCompletionNewParams{
		Model: shared.ChatModel(r.Model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(sys),
			openai.UserMessage(transcript),
		},
		ReasoningEffort: r.Cfg.GlobalReasoningEffort(),
	}
	llm.ApplyMaxResponseTokens(r.Cfg, &params)
	const sep = "================================================================================"
	summary, usage, err := llm.StreamText(ctx, r.Client, params, io.Discard, llm.StreamOpts{})
	if err != nil {
		return err
	}
	if r.Cfg.UsageStatsEnabled() {
		fmt.Fprintln(r.Out, termcolor.UsageTokensLine(usage.PromptTokens, usage.ReasoningTokens, usage.ResponseTokens, usage.TotalTokens, usage.OutputTPS, usage.TTFTSecs, usage.PromptTPS))
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return fmt.Errorf("empty summary from model")
	}
	var retainedBlock string
	if len(msgs) > 8 {
		retainedBlock = formatChatTranscript(msgs[len(msgs)-8:])
	} else {
		retainedBlock = formatChatTranscript(msgs)
	}
	body := compactSummaryBody(sep, summary, retainedBlock)
	r.Session.Messages = []chatstore.Message{{Role: "assistant", Content: body}}
	r.Session.LastMessageAt = time.Now()
	if err := chatstore.WriteSession(r.ProjHex, r.Session); err != nil {
		return err
	}
	fmt.Fprint(r.Out, "\033[2J\033[H")
	fmt.Fprintln(r.Out, body)
	fmt.Fprintln(r.Out, "Cronologia compattata: riassunto salvato; i messaggi precedenti sono stati sostituiti.")
	return nil
}
