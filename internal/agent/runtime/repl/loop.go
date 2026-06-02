package repl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/slash"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/multiline"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"

	readline "github.com/chzyer/readline"
)

type Loop struct {
	RL  *readline.Instance
	Out io.Writer
	Ctx context.Context

	CompleteEnv             replcomplete.ReplCompleteEnv
	FinishSessionLoad       func()
	RefreshPrompt           func()
	RefreshPromptContinue   func()
	HandleSlash             func(line string) error
	SlashDeps               func() commands.Deps
	OnUserMessage           func(line string) error
	ClipboardPasteForStdin  func() (tag string, ok bool)
	SaveClipboardImage      func() (seq int, err error)
}

func Run(loop *Loop) error {
	if loop == nil || loop.RL == nil {
		return fmt.Errorf("repl: nil loop or readline")
	}
	multiline.SetReplImagePaste(func() (string, bool) {
		if loop.ClipboardPasteForStdin == nil {
			return "", false
		}
		tag, ok := loop.ClipboardPasteForStdin()
		return tag, ok
	})
	defer multiline.SetReplImagePaste(nil)
	restoreInput := multiline.EnableReplInputModes(loop.RL.Stdout())
	defer restoreInput()
	var pendingMultiline []string
	cfg := loop.RL.Config.Clone()
	cfg.AutoComplete = replcomplete.NewReplCompleter(loop.CompleteEnv)
	cfg.Painter = imgDisplayPainter{}
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
		if newLine, newPos, ok := TryPasteImageAtCursor(loop.RL.Stderr(), loop.SaveClipboardImage, line, pos, key); ok {
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
			fmt.Fprint(loop.RL.Stdout(), "\x1b[1A\r\x1b[2K")
			if len(pendingMultiline) == 0 {
				loop.RefreshPrompt()
			} else {
				loop.RefreshPromptContinue()
			}
			rs := []rune(last)
			return rs, len(rs), true
		}
		return nil, 0, false
	})
	loop.RL.SetConfig(cfg)
	for {
		if loop.FinishSessionLoad != nil {
			loop.FinishSessionLoad()
		}
		if len(pendingMultiline) > 0 {
			loop.RefreshPromptContinue()
		} else {
			loop.RefreshPrompt()
		}
		line, err := loop.RL.Readline()
		if err != nil {
			switch {
			case errors.Is(err, io.EOF):
				logging.Log(logging.INFO_LOG_LEVEL, "interactive session ended (EOF)")
			case errors.Is(err, readline.ErrInterrupt):
				logging.Log(logging.INFO_LOG_LEVEL, "interactive session ended (Ctrl+C at prompt)")
				if loop.SlashDeps != nil {
					commands.ExitMessage(loop.SlashDeps())
				}
				return nil
			default:
				logging.Log(logging.ERROR_LOG_LEVEL, "readline failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			}
			return err
		}
		line, isSoftBreak := multiline.ParseMultilineControlRunes(line)
		if isSoftBreak {
			pendingMultiline = append(pendingMultiline, line)
			continue
		}
		if len(pendingMultiline) > 0 {
			pendingMultiline = append(pendingMultiline, line)
			line = strings.Join(pendingMultiline, "\n")
			pendingMultiline = nil
		}
		line = multiline.TrimMessageEdges(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "/") {
			if err := loop.HandleSlash(line); err != nil {
				if errors.Is(err, slash.ErrExitChat) {
					logging.Log(logging.INFO_LOG_LEVEL, "user requested exit from chat")
					return nil
				}
				commands.PrintSystemErr(loop.Out, err)
			}
			continue
		}
		if err := loop.OnUserMessage(line); err != nil {
			logging.Log(logging.ERROR_LOG_LEVEL, "onUserMessage failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			commands.PrintSystemErr(loop.Out, err)
		}
	}
}
