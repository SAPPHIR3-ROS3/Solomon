package agentruntime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	solomonagent "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/clipboard"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"

	readline "github.com/chzyer/readline"
)

type imgReplDisplayPainter struct{}

const replImagePasteKey = 22

func stripReplPasteTrigger(line []rune, pos int, key rune) ([]rune, int) {
	if pos <= 0 || key != replImagePasteKey || line[pos-1] != replImagePasteKey {
		return line, pos
	}
	return append(append([]rune(nil), line[:pos-1]...), line[pos:]...), pos - 1
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

func (r *Runtime) tryReplPasteImage(line []rune, pos int, key rune) ([]rune, int, bool) {
	if key != replImagePasteKey {
		return nil, 0, false
	}
	if !clipboard.HasImage() {
		return nil, 0, false
	}
	seq, _, err := r.saveReplClipboardImage()
	if err != nil {
		fmt.Fprintf(r.RL.Stderr(), "clipboard image paste failed: %v\n", err)
		return nil, 0, false
	}
	line, pos = stripReplPasteTrigger(line, pos, key)
	tag := llm.ImagePlaceholder(seq)
	newRunes := make([]rune, 0, len(line)+len(tag))
	newRunes = append(newRunes, line[:pos]...)
	newRunes = append(newRunes, []rune(tag)...)
	newRunes = append(newRunes, line[pos:]...)
	return newRunes, pos + len([]rune(tag)), true
}

func (imgReplDisplayPainter) Paint(line []rune, _ int) []rune {
	return []rune(termcolor.ColorizeImgTagsReplInput(string(line)))
}

func (r *Runtime) Run(ctx context.Context) error {
	logging.Log(logging.INFO_LOG_LEVEL, "interactive REPL started")
	r.mutateSession(func(s *chatstore.Session) {
		chatstore.FinishSessionLoad(s)
	})
	printWelcomeBanner(r.Out, r.Cfg, r.Model, r.ProjHex, r.ProjRoot, r.ReplShellFirst)
	go func() { r.InitMCP(ctx) }()
	if !config.NeedsOnboard(r.Cfg) {
		go commands.PrefetchSlashModelCatalog(ctx, r.Cfg, r.Out)
	}
	SetReplImagePaste(func() (string, bool) {
		seq, _, err := r.saveReplClipboardImage()
		if err != nil {
			fmt.Fprintf(r.RL.Stderr(), "clipboard image paste failed: %v\n", err)
			return "", false
		}
		return llm.ImagePlaceholder(seq), true
	})
	defer SetReplImagePaste(nil)
	restoreInput := enableReplInputModes(r.RL.Stdout())
	defer restoreInput()
	var pendingMultiline []string
	cfg := r.RL.Config.Clone()
	cfg.AutoComplete = NewReplCompleter(r.replCompleteEnv())
	cfg.Painter = imgReplDisplayPainter{}
	cfg.Listener = readline.FuncListener(func(line []rune, pos int, key rune) ([]rune, int, bool) {
		if key == readline.CharBackward && len(line) > 0 {
			if newPos := llm.JumpLeftOverImgTag(line, pos); newPos >= 0 {
				return line, newPos, true
			}
			return nil, 0, false
		}
		if key == readline.CharForward && len(line) > 0 {
			if newPos := llm.JumpRightOverImgTag(line, pos); newPos >= 0 {
				return line, newPos, true
			}
			return nil, 0, false
		}
		if newLine, newPos, ok := r.tryReplPasteImage(line, pos, key); ok {
			return newLine, newPos, true
		}
		if len(pendingMultiline) == 0 {
			return nil, 0, false
		}
		if line == nil {
			return nil, 0, false
		}
		if (key == readline.CharBackspace || key == readline.CharCtrlH) && len(line) == 0 {
			last := pendingMultiline[len(pendingMultiline)-1]
			pendingMultiline = pendingMultiline[:len(pendingMultiline)-1]
			fmt.Fprint(r.RL.Stdout(), "\x1b[1A\r\x1b[2K")
			if len(pendingMultiline) == 0 {
				r.refreshReadlinePrompt()
			} else {
				r.refreshReadlinePromptContinue()
			}
			rs := []rune(last)
			return rs, len(rs), true
		}
		return nil, 0, false
	})
	r.RL.SetConfig(cfg)
	for {
		r.mutateSession(func(s *chatstore.Session) {
			chatstore.FinishSessionLoad(s)
		})
		if len(pendingMultiline) > 0 {
			r.refreshReadlinePromptContinue()
		} else {
			r.refreshReadlinePrompt()
		}
		line, err := r.RL.Readline()
		if err != nil {
			switch {
			case errors.Is(err, io.EOF):
				logging.Log(logging.INFO_LOG_LEVEL, "interactive session ended (EOF)")
			case errors.Is(err, readline.ErrInterrupt):
				logging.Log(logging.INFO_LOG_LEVEL, "interactive session ended (Ctrl+C at prompt)")
				commands.ExitMessage(r.slashDeps(ctx))
				return nil
			default:
				logging.Log(logging.ERROR_LOG_LEVEL, "readline failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			}
			return err
		}
		line, isSoftBreak := parseMultilineControlRunes(line)
		if isSoftBreak {
			pendingMultiline = append(pendingMultiline, line)
			continue
		}
		if len(pendingMultiline) > 0 {
			pendingMultiline = append(pendingMultiline, line)
			line = strings.Join(pendingMultiline, "\n")
			pendingMultiline = nil
		}
		line = trimMessageEdges(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "/") {
			if err := r.handleSlash(ctx, line); err != nil {
				if errors.Is(err, solomonagent.ErrExitChat) {
					logging.Log(logging.INFO_LOG_LEVEL, "user requested exit from chat")
					return nil
				}
				commands.PrintSystemErr(r.Out, err)
			}
			continue
		}
		if err := r.onUserMessage(ctx, line, true); err != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, "onUserMessage failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			commands.PrintSystemErr(r.Out, err)
		}
	}
}
