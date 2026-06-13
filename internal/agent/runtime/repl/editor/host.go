package editor

import (
	"io"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
	readline "github.com/chzyer/readline"
)

type Host struct {
	RL                     *readline.Instance
	Out                    io.Writer
	InputInterrupt         <-chan struct{}
	CompleteEnv            replcomplete.ReplCompleteEnv
	PromptPrimary          func() string
	PromptContinue         func() string
	ClipboardPasteForStdin func() (tag string, ok bool)
}
