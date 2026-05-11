package agentruntime

import (
	"context"
	"fmt"
	"os"

	solomonagent "github.com/SAPPHIR3-ROS3/Solomon/internal/agent"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
)

func (r *Runtime) handleSlash(ctx context.Context, line string) error {
	return solomonagent.SlashDispatch(r.slashDeps(ctx), line)
}

func (r *Runtime) slashDeps(ctx context.Context) commands.Deps {
	return commands.Deps{
		Ctx:   ctx,
		Out:   r.Out,
		Stdin: os.Stdin,
		ReadLine: func(prompt string) (string, error) {
			if r.RL == nil {
				return "", fmt.Errorf("/models line input unavailable")
			}
			prev := r.RL.Config.Prompt
			r.RL.SetPrompt(prompt)
			line, err := r.RL.Readline()
			r.RL.SetPrompt(prev)
			return line, err
		},
		Cfg: r.Cfg,
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
		SubmitUserMessage: func(s string) error { return r.onUserMessage(ctx, s, false) },

		PrintWelcomeBanner: func() {
			printWelcomeBanner(r.Out, r.Cfg, r.Model, r.ProjHex, r.ProjRoot, r.ReplShellFirst)
		},

		PersistSession: r.persistSession,

		CheckpointGoto: r.ApplyGotoCheckpoint,

		GetReplShellFirst: func() bool { return r.ReplShellFirst },
		SetReplShellFirst: func(v bool) { r.ReplShellFirst = v },
	}
}
