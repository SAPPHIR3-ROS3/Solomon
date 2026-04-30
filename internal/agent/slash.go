package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"solomon/internal/agent/commands"
	"solomon/internal/chatstore"
	"solomon/internal/config"
	"solomon/internal/logging"
	"solomon/internal/skills"
)

var ErrExitChat = errors.New("exit chat")

func (r *Runtime) handleSlash(ctx context.Context, line string) error {
	return SlashDispatch(r.slashDeps(ctx), line)
}

func (r *Runtime) slashDeps(ctx context.Context) commands.Deps {
	return commands.Deps{
		Ctx:     ctx,
		Out:     r.Out,
		Stdin:   os.Stdin,
		Cfg:     r.Cfg,
		SaveCfg: func() error { return config.Save(r.Cfg) },

		ProjHex:  r.ProjHex,
		ProjRoot: r.ProjRoot,

		Session:    func() *chatstore.Session { return r.Session },
		SetSession: func(s *chatstore.Session) { r.Session = s },

		SetMode: func(m string) { r.Mode = m },
		GetMode: func() string { return r.Mode },

		ApplyCurrentModel: r.ApplyCurrentModel,
		Model:             func() string { return r.Model },
		Provider:          func() *config.Provider { return r.Prov },

		CompactionThresholdTokens: func() int64 { return r.CompactionThresholdTokens },
		SetCompactionThresholdTokens: func(n int64) {
			r.CompactionThresholdTokens = n
			r.Cfg.CompactionThresholdTokens = n
			_ = config.Save(r.Cfg)
		},

		Client: r.Client,

		ResetReadlineHistory: func() { r.RL.ResetHistory() },
		AppendReadlineHistory: func(line string) error {
			return r.RL.SaveHistory(line)
		},
		PrefillInput: func(s string) {
			r.RL.Operation.SetBuffer(s)
		},
		SubmitUserMessage: func(s string) error { return r.onUserMessage(ctx, s) },

		PrintWelcomeBanner: func() {
			printWelcomeBanner(r.Out, r.Cfg, r.Model, r.ProjHex, r.ProjRoot)
		},
	}
}

func splitSlashArgs(line string) []string {
	line = strings.TrimSpace(line)
	var fields []string
	for len(line) > 0 {
		if line[0] == '"' {
			line = line[1:]
			var b strings.Builder
			for len(line) > 0 {
				if line[0] == '\\' && len(line) > 1 {
					b.WriteByte(line[1])
					line = line[2:]
					continue
				}
				if line[0] == '"' {
					line = line[1:]
					break
				}
				b.WriteByte(line[0])
				line = line[1:]
			}
			fields = append(fields, b.String())
			line = strings.TrimLeft(line, " \t")
			continue
		}
		i := strings.IndexAny(line, " \t")
		if i < 0 {
			fields = append(fields, line)
			break
		}
		fields = append(fields, line[:i])
		line = strings.TrimLeft(line[i:], " \t")
	}
	return fields
}

func slashCommandName(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	t := strings.TrimSpace(parts[0])
	t = strings.TrimPrefix(t, "/")
	t = strings.TrimSpace(t)
	t = strings.Trim(t, "\ufeff\u200b\u200c\u200d")
	return strings.ToLower(t)
}

func SlashDispatch(d commands.Deps, line string) error {
	parts := splitSlashArgs(line)
	if len(parts) == 0 {
		return nil
	}
	name := slashCommandName(parts)
	if name != "" {
		logging.Log(logging.INFO_LOG_LEVEL, "slash dispatch", logging.LogOptions{Params: map[string]any{"command": name}})
	}
	switch name {
	case "plan":
		return commands.Plan(d)
	case "build":
		return commands.Build(d)
	case "clear":
		return commands.Clear(d)
	case "exec":
		if d.SubmitUserMessage == nil {
			return fmt.Errorf("/exec unavailable")
		}
		if len(parts) < 2 {
			return fmt.Errorf("usage: /exec <prompt> or /exec \"prompt with spaces\"")
		}
		return d.SubmitUserMessage(strings.Join(parts[1:], " "))
	case "log":
		return commands.SlashLog(d, parts)
	case "reasoning":
		return commands.Reasoning(d, parts)
	case "timeout":
		return commands.Timeout(d, parts)
	case "stats":
		return commands.Stats(d)
	case "thinking":
		return commands.Thinking(d, parts)
	case "max_response":
		return commands.MaxResponse(d, parts)
	case "threshold":
		return commands.Threshold(d, parts)
	case "models":
		return commands.SlashModels(d)
	case "connect":
		return commands.Connect(d)
	case "new":
		return commands.NewChat(d)
	case "resume":
		return commands.Resume(d, parts[1:])
	case "summarize", "compact":
		return commands.Summarize(d)
	case "exit", "quit":
		commands.ExitMessage(d)
		return ErrExitChat
	case "language":
		return commands.Language(d, parts)
	case "legacytools", "legacy":
		return commands.LegacyTools(d, parts)
	case "add":
		if len(parts) < 2 {
			return fmt.Errorf(`usage: /add npx ... | skills.sh | skill <.md> [name] [scope]`)
		}
		return commands.Add(d, parts[1:])
	case "skills":
		return commands.Skills(d)
	case "remove":
		if len(parts) < 2 {
			return fmt.Errorf(`usage: /remove skill <name>`)
		}
		return commands.Remove(d, parts[1:])
	case "help":
		commands.WriteHelp(d.Out, d.ProjHex, d.ProjRoot)
		return nil
	default:
		e, err := skills.LookupSkillBySlashCommand(name, d.ProjHex, d.ProjRoot)
		if err != nil {
			return err
		}
		if e != nil {
			return commands.RunSkillSlash(d, *e)
		}
		return fmt.Errorf("unknown command /%s (try /help)", name)
	}
}
