package commands

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

var ErrBuiltinExitChat = errors.New("exit chat")

type slashBuiltin struct {
	keys     []string
	helpCol  string
	detail   string
	dispatch func(Deps, []string) error
}

var (
	slashBuiltinsMu sync.Mutex
	slashBuiltins   []slashBuiltin
)

func getSlashBuiltins() []slashBuiltin {
	slashBuiltinsMu.Lock()
	defer slashBuiltinsMu.Unlock()
	if slashBuiltins != nil {
		return slashBuiltins
	}
	slashBuiltins = []slashBuiltin{
		{[]string{"plan"}, "/plan", "planning tools only", func(d Deps, parts []string) error { return Plan(d) }},
		{[]string{"build"}, "/build", "build tools (shell, files, subagent)", func(d Deps, parts []string) error { return Build(d) }},
		{[]string{"clear"}, "/clear", "clear terminal (ANSI)", func(d Deps, parts []string) error { return Clear(d) }},
		{[]string{"cleansessioncache"}, "/cleansessioncache", "/cleansessioncache — drop broken pasted PNG paths and strip orphaned [img-*] from transcript", func(d Deps, parts []string) error {
			return CleanSessionCache(d)
		}},
		{[]string{"terminal"}, "/terminal", "/terminal | /terminal on|off — shell-first input: plain lines = shell; prefix ! = AI message (default is the opposite)", func(d Deps, parts []string) error { return Terminal(d, parts) }},
		{[]string{"exec"}, "/exec", "/exec <prompt> | /exec \"prompt with spaces\" — send one user message", func(d Deps, parts []string) error {
			if d.SubmitUserMessage == nil {
				return fmt.Errorf("/exec unavailable")
			}
			if len(parts) < 2 {
				return fmt.Errorf("usage: /exec <prompt> or /exec \"prompt with spaces\"")
			}
			return d.SubmitUserMessage(strings.Join(parts[1:], " "))
		}},
		{[]string{"log"}, "/log", "/log {error|warning|info|debug|result} visible log verbosity", func(d Deps, parts []string) error { return SlashLog(d, parts) }},
		{[]string{"reasoning"}, "/reasoning", "/reasoning | /reasoning {none|low|med|high} main chat; subagent always none", func(d Deps, parts []string) error { return Reasoning(d, parts) }},
		{[]string{"timeout"}, "/timeout", "/timeout <minutes> subagent segment (1–180)", func(d Deps, parts []string) error { return Timeout(d, parts) }},
		{[]string{"stats"}, "/stats", "toggle token usage line after assistant turns (saved)", func(d Deps, parts []string) error { return Stats(d) }},
		{[]string{"thinking"}, "/thinking", "/thinking toggles preview; /thinking on|off streamed reasoning (dim gray); tool echoes (yellow)", func(d Deps, parts []string) error { return Thinking(d, parts) }},
		{[]string{"fast"}, "/fast", "/fast | /fast on|off Cursor fast mode (saved)", func(d Deps, parts []string) error { return Fast(d, parts) }},
		{[]string{"max_response"}, "/max_response", "/max_response | /max_response <n> assistant output cap (tokens, n>=1)", func(d Deps, parts []string) error { return MaxResponse(d, parts) }},
		{[]string{"threshold"}, "/threshold", "/threshold | /threshold <n> auto /summarize when prompt_tokens >= n (n>=32768; default 131072; needs API usage)", func(d Deps, parts []string) error { return Threshold(d, parts) }},
		{[]string{"models"}, "/models", "list models and switch current model", func(d Deps, parts []string) error { return SlashModels(d) }},
		{[]string{"connect"}, "/connect", "connect ChatGPT Sub, OpenAI-compatible API, Anthropic API key, Claude Sub (coming soon), or Cursor API; then pick model", func(d Deps, parts []string) error { return Connect(d) }},
		{[]string{"new"}, "/new", "start a new chat session (empty transcript; prior chat stays saved on disk)", func(d Deps, parts []string) error { return NewChat(d) }},
		{[]string{"temp"}, "/temp", "/temp — empty chat only: in-memory session (not saved; like solomon temp exec)", func(d Deps, parts []string) error { return TempChat(d) }},
		{[]string{"resume"}, "/resume", "/resume | /resume last | /resume <id|title>", func(d Deps, parts []string) error { return Resume(d, parts[1:]) }},
		{[]string{"summarize", "compact"}, "/summarize, /compact", "summarize full chat; summary + last 8 msgs; then /clear", func(d Deps, parts []string) error { return Summarize(d) }},
		{[]string{"exit", "quit"}, "/exit, /quit", "exit and show how to resume", func(d Deps, parts []string) error {
			ExitMessage(d)
			return ErrBuiltinExitChat
		}},
		{[]string{"name"}, "/name", "/name | /name <name> | /name clear — user name (saved; system prompt)", func(d Deps, parts []string) error { return Name(d, parts) }},
		{[]string{"language"}, "/language", "/language | /language <language> | /language clear — reply language (default English; saved; system prompt)", func(d Deps, parts []string) error { return Language(d, parts) }},
		{[]string{"legacytools", "legacy"}, "/legacytools", "/legacytools on|off | /legacytools force on|off — legacy XML tools (saved)", func(d Deps, parts []string) error { return LegacyTools(d, parts) }},
		{[]string{"add"}, "/add", "/add rule <phrase> | /add projectrule <phrase> | npx skills add ... | https://skills.sh/... | skill <path/to/.md> [name] [global|project|local]", func(d Deps, parts []string) error {
			if len(parts) < 2 {
				return fmt.Errorf(`usage: /add rule <phrase> | /add projectrule <phrase> | npx ... | skills.sh | skill <.md> [name] [scope]`)
			}
			return Add(d, parts[1:])
		}},
		{[]string{"skills"}, "/skills", "/skills — list installed skills (Local → Project → Global; empty sections omitted)", func(d Deps, parts []string) error { return Skills(d) }},
		{[]string{"rules"}, "/rules", "/rules — list custom global and project rules", func(d Deps, parts []string) error { return Rules(d) }},
		{[]string{"instructions"}, "/instructions", "/instructions — show global AGENTS.md loaded for the system prompt", func(d Deps, parts []string) error { return Instructions(d) }},
		{[]string{"remove"}, "/remove", "/remove rule <N> | /remove projectrule <N> | /remove skill <name>", func(d Deps, parts []string) error {
			if len(parts) < 2 {
				return fmt.Errorf(`usage: /remove rule <N> | /remove projectrule <N> | /remove skill <name>`)
			}
			return Remove(d, parts[1:])
		}},
		{[]string{"version"}, "/version", "print installed Solomon version", func(d Deps, parts []string) error { return Version(d) }},
		{[]string{"update"}, "/update", "check GitHub releases; clear screen and refresh welcome banner (does not install)", func(d Deps, parts []string) error { return Update(d) }},
		{[]string{"autoupdate"}, "/autoupdate", "/autoupdate | /autoupdate on|off — auto-install newer releases (config.toml)", func(d Deps, parts []string) error { return AutoUpdate(d, parts) }},
		{[]string{"upgrade"}, "/upgrade", "/upgrade — install the available release using your OS install command", func(d Deps, parts []string) error { return Upgrade(d) }},
		{[]string{"onboard"}, "/onboard", "run setup wizard (overwrites first-setup fields)", func(d Deps, parts []string) error { return Onboard(d) }},
		{[]string{"configbackup"}, "/configbackup", "copy config.toml to ~/.solomon/backup/config.toml.<isodate>.bak", func(d Deps, parts []string) error { return ConfigBackup(d) }},
		{[]string{"help"}, "/help", "this list", func(d Deps, parts []string) error {
			WriteHelp(d.Out, d.ProjHex, d.ProjRoot)
			return nil
		}},
		{[]string{"goto"}, "/goto", "/goto <checkpoint-id> rewind transcript to checkpoint (e.g. 5, #006a); fork suffix on new lines", func(d Deps, parts []string) error { return SlashGoto(d, parts) }},
		{[]string{"checkpoint"}, "/checkpoint", "print current checkpoint tag", func(d Deps, parts []string) error {
			SlashCheckpointAck(d)
			return nil
		}},
		{[]string{"mcp"}, "/mcp", "list MCP servers from config (URLs redacted)", func(d Deps, parts []string) error { return SlashMCP(d) }},
		{[]string{"integrations"}, "/integrations", "Cursor API sidecar URL, health, and install path", func(d Deps, parts []string) error { return SlashIntegrations(d) }},
		{[]string{"testweb"}, "/testweb", "test web search config; OK or NOT OK then duckduckgo fallback", func(d Deps, parts []string) error { return TestWeb(d) }},
	}
	return slashBuiltins
}

func Terminal(d Deps, parts []string) error {
	if d.GetReplShellFirst == nil || d.SetReplShellFirst == nil {
		return fmt.Errorf("/terminal unavailable")
	}
	if len(parts) < 2 {
		next := !d.GetReplShellFirst()
		d.SetReplShellFirst(next)
		state := "off"
		if next {
			state = "on"
		}
		PrintSystemf(d.Out, "shell-first REPL: %s (plain line → shell; !… → AI)", state)
		if d.PrintWelcomeBanner != nil {
			d.PrintWelcomeBanner()
		}
		return nil
	}
	sw := strings.ToLower(parts[1])
	switch sw {
	case "on", "yes", "true", "1":
		d.SetReplShellFirst(true)
	case "off", "no", "false", "0":
		d.SetReplShellFirst(false)
	default:
		return fmt.Errorf("usage: /terminal | /terminal on|off")
	}
	state := "off"
	if d.GetReplShellFirst() {
		state = "on"
	}
	PrintSystemf(d.Out, "shell-first REPL: %s (plain line → shell; !… → AI)", state)
	if d.PrintWelcomeBanner != nil {
		d.PrintWelcomeBanner()
	}
	return nil
}

func DispatchBuiltinSlash(d Deps, parts []string, name string) (matched bool, err error) {
	if name == "" {
		return false, nil
	}
	tab := getSlashBuiltins()
	for i := range tab {
		b := &tab[i]
		for _, k := range b.keys {
			if k == name {
				return true, b.dispatch(d, parts)
			}
		}
	}
	return false, nil
}
