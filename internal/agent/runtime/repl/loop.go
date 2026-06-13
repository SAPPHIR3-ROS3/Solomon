package repl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/multiline"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl/editor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/slash"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"

	readline "github.com/chzyer/readline"
)

type Loop struct {
	RL  *readline.Instance
	Out io.Writer
	Ctx context.Context

	InputInterrupt         <-chan struct{}
	PrepareStartupNotice   func()
	TakeStartupNotice      func() bool
	CompleteEnv            replcomplete.ReplCompleteEnv
	FinishSessionLoad      func()
	PromptPrimary          func() string
	PromptContinue         func() string
	HandleSlash            func(line string) error
	SlashDeps              func() commands.Deps
	OnUserMessage          func(line string) error
	ClipboardPasteForStdin func() (tag string, ok bool)
	SaveClipboardImage     func() (tag string, err error)
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
	history := editor.NewHistory()
	for {
		if loop.FinishSessionLoad != nil {
			loop.FinishSessionLoad()
		}
		if loop.PrepareStartupNotice != nil {
			loop.PrepareStartupNotice()
		}
		line, err := editor.ReadMultiline(editorHostFromLoop(loop), history)
		if errors.Is(err, ErrInputInterrupted) {
			if loop.TakeStartupNotice != nil {
				loop.TakeStartupNotice()
			}
			continue
		}
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
		line, _ = multiline.ParseMultilineControlRunes(line)
		line = multiline.TrimMessageEdges(line)
		if line == "" {
			continue
		}
		history.Add(line, loop.CompleteEnv.ReplShellFirst)
		if strings.HasPrefix(line, "/") {
			if err := loop.HandleSlash(line); err != nil {
				if errors.Is(err, slash.ErrExitChat) {
					logging.Log(logging.INFO_LOG_LEVEL, "user requested exit from chat")
					return nil
				}
				if errors.Is(err, slash.ErrRestartSolomon) {
					return slash.ErrRestartSolomon
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
