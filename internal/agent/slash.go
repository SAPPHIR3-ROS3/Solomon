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

		ProjHex: r.ProjHex,

		Session:    func() *chatstore.Session { return r.Session },
		SetSession: func(s *chatstore.Session) { r.Session = s },

		SetMode: func(m string) { r.Mode = m },
		GetMode: func() string { return r.Mode },

		ApplyCurrentModel: r.ApplyCurrentModel,
		Model:             func() string { return r.Model },
		Provider:          func() *config.Provider { return r.Prov },

		Client: r.Client,

		ResetReadlineHistory: func() { r.RL.ResetHistory() },
		AppendReadlineHistory: func(line string) error {
			return r.RL.SaveHistory(line)
		},
	}
}

func SlashDispatch(d commands.Deps, line string) error {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}
	name := strings.TrimPrefix(parts[0], "/")
	switch name {
	case "plan":
		return commands.Plan(d)
	case "build":
		return commands.Build(d)
	case "clear":
		return commands.Clear(d)
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
	case "models":
		return commands.SlashModels(d)
	case "connect":
		return commands.Connect(d)
	case "resume":
		return commands.Resume(d, parts[1:])
	case "summarize", "compact":
		return commands.Summarize(d)
	case "exit", "quit":
		commands.ExitMessage(d)
		return ErrExitChat
	case "language":
		return commands.Language(d, parts)
	case "help":
		commands.WriteHelp(d.Out)
		return nil
	default:
		return fmt.Errorf("unknown command /%s (try /help)", name)
	}
}
