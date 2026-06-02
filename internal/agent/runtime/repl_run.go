package agentruntime

import (
	"context"
	"fmt"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
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
	logging.Log(logging.INFO_LOG_LEVEL, "interactive REPL started")
	r.mutateSession(func(s *chatstore.Session) {
		chatstore.FinishSessionLoad(s)
	})
	repl.PrintWelcomeBanner(r.Out, r.Cfg, r.Model, r.ProjHex, r.ProjRoot, r.ReplShellFirst)
	go func() { r.InitMCP(ctx) }()
	if !config.NeedsOnboard(r.Cfg) {
		go commands.PrefetchSlashModelCatalog(ctx, r.Cfg, r.Out)
	}
	return repl.Run(&repl.Loop{
		RL:                     r.RL,
		Out:                    r.Out,
		Ctx:                    ctx,
		CompleteEnv:            replcomplete.EnvFrom(r),
		FinishSessionLoad:      r.finishReplSessionLoad,
		RefreshPrompt:          r.refreshReadlinePrompt,
		RefreshPromptContinue:  r.refreshReadlinePromptContinue,
		HandleSlash:            func(line string) error { return r.handleSlash(ctx, line) },
		SlashDeps:              func() commands.Deps { return r.slashDeps(ctx) },
		OnUserMessage:          func(line string) error { return r.onUserMessage(ctx, line, true) },
		ClipboardPasteForStdin: r.replClipboardPasteTag,
		SaveClipboardImage:     r.saveReplClipboardImageSeq,
	})
}

func (r *Runtime) finishReplSessionLoad() {
	r.mutateSession(func(s *chatstore.Session) {
		chatstore.FinishSessionLoad(s)
	})
}

func (r *Runtime) replClipboardPasteTag() (string, bool) {
	seq, _, err := r.saveReplClipboardImage()
	if err != nil {
		if r.RL != nil {
			fmt.Fprintf(r.RL.Stderr(), "clipboard image paste failed: %v\n", err)
		}
		return "", false
	}
	return llm.ImagePlaceholder(seq), true
}

func (r *Runtime) saveReplClipboardImageSeq() (int, error) {
	seq, _, err := r.saveReplClipboardImage()
	return seq, err
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
		seq = s.ImageSeq
		s.ImageSeq++
		chatID = s.ID
	})
	dir, err := paths.ChatImagesDir(r.ProjHex)
	if err != nil {
		return 0, "", err
	}
	path, err = clipboard.PasteImage(dir, chatID, seq)
	if err != nil {
		return 0, "", err
	}
	r.mutateSession(func(s *chatstore.Session) {
		if s.ImageFiles == nil {
			s.ImageFiles = make(map[int]string)
		}
		s.ImageFiles[seq] = path
		s.LastMessageAt = time.Now()
	})
	return seq, path, nil
}
