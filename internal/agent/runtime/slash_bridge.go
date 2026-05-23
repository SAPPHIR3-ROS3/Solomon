package agentruntime

import (
	"context"
	"fmt"
	"os"

	solomonagent "github.com/SAPPHIR3-ROS3/Solomon/internal/agent"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"

	readline "github.com/chzyer/readline"
)

func StdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func ReadlinePrompt(rl *readline.Instance, prompt string) (string, error) {
	if rl == nil {
		return "", fmt.Errorf("readline unavailable")
	}
	prev := rl.Config.Prompt
	rl.SetPrompt(prompt)
	line, err := rl.Readline()
	rl.SetPrompt(prev)
	return line, err
}

func NewREPLReadline(defaultPrompt string) (*readline.Instance, func(string) (string, error), error) {
	if !StdinIsTerminal() {
		return nil, nil, nil
	}
	rl, err := readline.NewEx(&readline.Config{
		Prompt:       defaultPrompt,
		Stdin:        NewMultilineStdin(PlatformStdin()),
		AutoComplete: NewReplCompleter(ReplCompleteEnv{}),
	})
	if err != nil {
		return nil, nil, err
	}
	fn := func(prompt string) (string, error) {
		return ReadlinePrompt(rl, prompt)
	}
	return rl, fn, nil
}

func (r *Runtime) promptIO() config.PromptIO {
	pio := config.PromptIO{Stdin: os.Stdin, Out: r.Out}
	if r.RL != nil {
		rl := r.RL
		pio.ReadLine = func(prompt string) (string, error) {
			return ReadlinePrompt(rl, prompt)
		}
	}
	return pio
}

func (r *Runtime) handleSlash(ctx context.Context, line string) error {
	return solomonagent.SlashDispatch(r.slashDeps(ctx), line)
}

func (r *Runtime) slashDeps(ctx context.Context) commands.Deps {
	pio := r.promptIO()
	return commands.Deps{
		Ctx:      ctx,
		Out:      pio.Out,
		Stdin:    pio.Stdin,
		ReadLine: pio.ReadLine,
		Cfg:      r.Cfg,
		SaveCfg:  func() error { return config.Save(r.Cfg) },

		ProjHex:  r.ProjHex,
		ProjRoot: r.ProjRoot,

		Session: func() *chatstore.Session { return r.Session },
		SetSession: func(s *chatstore.Session) {
			r.chatPersistMu.Lock()
			r.Session = s
			r.sessionFileCreated = s != nil && s.ID != ""
			r.chatPersistMu.Unlock()
		},
		MutateSession: r.mutateSession,

		SetMode: func(m string) { r.Mode = m },
		GetMode: func() string { return r.Mode },

		ApplyCurrentModel: r.ApplyCurrentModel,
		Model:             func() string { return r.Model },
		Provider:          func() *config.Provider { return r.Prov },

		CompactionThresholdTokens: func() int64 { return r.CompactionThresholdTokens },
		SetCompactionThresholdTokens: func(n int64) {
			r.CompactionThresholdTokens = n
			r.Cfg.CompactionThresholdTokens = n
			if err := config.Save(r.Cfg); err != nil {
				logging.Log(logging.WARNING_LOG_LEVEL, "save config after compaction threshold change failed", logging.LogOptions{Params: map[string]any{"err": err.Error(), "threshold": n}})
			}
		},

		Client:  r.Client,
		Backend: r.Backend,

		ResetReadlineHistory: func() {
			if r.RL != nil {
				r.RL.ResetHistory()
			}
		},
		AppendReadlineHistory: func(line string) error {
			if r.RL == nil {
				return nil
			}
			return r.RL.SaveHistory(line)
		},
		PrefillInput: func(s string) {
			if r.RL != nil {
				r.RL.Operation.SetBuffer(s)
			}
		},
		SubmitUserMessage:        func(s string) error { return r.onUserMessage(ctx, s, false) },
		SubmitVisibleUserMessage: func(visible, api string) error { return r.onUserMessageWithAPIContent(ctx, visible, api, false) },

		PrintWelcomeBanner: func() {
			printWelcomeBanner(r.Out, r.Cfg, r.Model, r.ProjHex, r.ProjRoot, r.ReplShellFirst)
		},

		PersistSession: r.persistSession,

		CheckpointGoto: r.ApplyGotoCheckpoint,

		GetReplShellFirst: func() bool { return r.ReplShellFirst },
		SetReplShellFirst: func(v bool) { r.ReplShellFirst = v },

		GetEphemeralSession: func() bool { return r.EphemeralSession },
		SetEphemeralSession: func(v bool) { r.EphemeralSession = v },
	}
}
