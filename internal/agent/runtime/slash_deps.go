package agentruntime

import (
	"context"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/slash"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func (r *Runtime) promptIO() config.PromptIO {
	pio := config.PromptIO{Stdin: os.Stdin, Out: r.Out}
	if r.RL != nil {
		rl := r.RL
		pio.ReadLine = func(prompt string) (string, error) {
			return repl.Prompt(rl, prompt)
		}
	}
	return pio
}

func (r *Runtime) handleSlash(ctx context.Context, line string) error {
	return slash.Dispatch(r.slashDeps(ctx), line)
}

func (r *Runtime) slashDeps(ctx context.Context) commands.Deps {
	pio := r.promptIO()
	return commands.Deps{
		Ctx:      ctx,
		Out:      pio.Out,
		Stdin:    pio.Stdin,
		ReadLine: pio.ReadLine,
		Cfg:      r.Cfg,
		SaveCfg: func() error {
			if err := config.Save(r.Cfg); err != nil {
				return err
			}
			commands.InvalidateAndPrefetchSlashModelCatalog(ctx, r.Cfg, r.Out)
			return nil
		},

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
			repl.PrintWelcomeBanner(r.Out, r.Cfg, r.Model, r.ProjHex, r.ProjRoot, r.ReplShellFirst, r.cachedUpdateNotice())
		},
		CheckForUpdate: func(force bool) (*updater.Notice, error) {
			return r.refreshUpdateCheck(ctx, force)
		},
		InstallUpdate: func(tag string) error {
			r.setPendingUpdateTag(tag)
			return updater.RunSystemInstall(ctx, tag, nil)
		},

		PersistSession: r.persistSession,

		CheckpointGoto:   r.ApplyGotoCheckpoint,
		CheckpointRewind: r.ApplyRewindCheckpoint,

		GetReplShellFirst: func() bool { return r.ReplShellFirst },
		SetReplShellFirst: func(v bool) { r.ReplShellFirst = v },

		GetEphemeralSession: func() bool { return r.EphemeralSession },
		SetEphemeralSession: func(v bool) { r.EphemeralSession = v },

		Research: r,
	}
}
