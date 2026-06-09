package agentruntime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/slash"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/multiline"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/clipboard"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"

	readline "github.com/chzyer/readline"
)

func NewREPLReadline(defaultPrompt string) (*readline.Instance, func(string) (string, error), error) {
	return repl.NewReadline(defaultPrompt)
}

func ReadlinePrompt(rl *readline.Instance, prompt string) (string, error) {
	return repl.Prompt(rl, prompt)
}

func StdinIsTerminal() bool {
	return repl.StdinIsTerminal()
}

func (r *Runtime) Run(ctx context.Context) error {
	multiline.EnsureCookedTTY()
	multiline.FlushStdin()
	defer multiline.EnsureCookedTTY()
	logging.Log(logging.INFO_LOG_LEVEL, "interactive REPL started")
	r.mutateSession(func(s *chatstore.Session) {
		chatstore.FinishSessionLoad(s)
	})
	notice, _ := r.refreshUpdateCheck(ctx, false)
	bannerNotice := notice
	if notice != nil && r.Cfg != nil && r.Cfg.AutoUpdateEnabled() {
		bannerNotice = nil
	}
	repl.PrintWelcomeBanner(r.Out, r.Cfg, r.Model, r.ProjHex, r.ProjRoot, r.ReplShellFirst, bannerNotice)
	if tag, ok := r.tryAutoUpdateInstall(ctx); ok {
		r.exitForUpdateRestart(fmt.Sprintf("autoupdate: installing %s...", tag), tag)
		return nil
	}
	go func() { r.InitMCP(ctx) }()
	if !config.NeedsOnboard(r.Cfg) {
		go commands.PrefetchSlashModelCatalog(ctx, r.Cfg, r.Out)
	}
	err := repl.Run(&repl.Loop{
		RL:                     r.RL,
		Out:                    r.Out,
		Ctx:                    ctx,
		CompleteEnv:            replcomplete.EnvFrom(r),
		FinishSessionLoad:      r.finishReplSessionLoad,
		PromptPrimary:          r.readlinePromptPrimary,
		PromptContinue:         r.readlinePromptContinue,
		HandleSlash:            func(line string) error { return r.handleSlash(ctx, line) },
		SlashDeps:              func() commands.Deps { return r.slashDeps(ctx) },
		OnUserMessage:          func(line string) error { return r.onUserMessage(ctx, line, true) },
		ClipboardPasteForStdin: r.replClipboardPasteTag,
		SaveClipboardImage:     r.saveReplClipboardImageTag,
	})
	if errors.Is(err, slash.ErrRestartSolomon) {
		tag := r.takePendingUpdateTag()
		lead := ""
		if tag != "" {
			lead = fmt.Sprintf("Installing %s...", tag)
		}
		r.exitForUpdateRestart(lead, tag)
		return nil
	}
	return err
}

func (r *Runtime) shutdownForUpdateRestart(leadLine string) {
	_ = r.persistSession()
	if r.RL != nil {
		r.RL.Clean()
		_ = r.RL.Terminal.ExitRawMode()
	}
	multiline.WriteTerminalModeSequences(multiline.BracketedPasteDisable + multiline.MouseReportDisable)
	multiline.EnsureCookedTTY()
	lines := []string{
		"Installing update and restarting in this terminal...",
	}
	if strings.TrimSpace(leadLine) != "" {
		lines = append([]string{leadLine}, lines...)
	}
	commands.PrintSystem(r.Out, strings.Join(lines, "\n"))
	if r.RL != nil {
		_ = r.RL.Close()
	}
}

func (r *Runtime) finishReplSessionLoad() {
	var repaired bool
	r.mutateSession(func(s *chatstore.Session) {
		repaired = chatstore.FinishSessionLoad(s)
	})
	if repaired {
		_ = r.persistSession()
	}
}

func (r *Runtime) replClipboardPasteTag() (string, bool) {
	tag, err := r.saveReplClipboardImageTag()
	if err != nil {
		if r.RL != nil {
			fmt.Fprintf(r.RL.Stderr(), "clipboard image paste failed: %v\n", err)
		}
		return "", false
	}
	return tag, true
}

func (r *Runtime) saveReplClipboardImageTag() (string, error) {
	seq, path, err := r.saveReplClipboardImage()
	if err != nil {
		return "", err
	}
	return llm.ImagePlaceholder(seq, path), nil
}

func (r *Runtime) saveReplClipboardImage() (seq int, path string, err error) {
	if !clipboard.HasImage() {
		return 0, "", clipboard.ErrNoImage
	}
	var chatID string
	r.mutateSession(func(s *chatstore.Session) {
		if s.ID == "" {
			s.ID = chatstore.NewPlaceholderChatID(time.Now())
		}
		chatID = s.ID
	})
	dir, err := paths.ChatImagesDir(r.ProjHex)
	if err != nil {
		return 0, "", err
	}
	r.mutateSession(func(s *chatstore.Session) {
		seq = s.ImageSeq
	})
	path, err = clipboard.PasteImage(dir, chatID, seq)
	if err != nil {
		return 0, "", err
	}
	r.mutateSession(func(s *chatstore.Session) {
		s.ImageSeq++
		if s.ImageFiles == nil {
			s.ImageFiles = make(map[int]string)
		}
		s.ImageFiles[seq] = path
		s.LastMessageAt = time.Now()
	})
	return seq, path, nil
}
