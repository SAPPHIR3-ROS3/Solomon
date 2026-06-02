package repl

import (
	"fmt"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/multiline"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"

	readline "github.com/chzyer/readline"
)

func StdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func Prompt(rl *readline.Instance, prompt string) (string, error) {
	if rl == nil {
		return "", fmt.Errorf("readline unavailable")
	}
	prev := rl.Config.Prompt
	rl.SetPrompt(prompt)
	line, err := rl.Readline()
	rl.SetPrompt(prev)
	return line, err
}

func NewReadline(defaultPrompt string) (*readline.Instance, func(string) (string, error), error) {
	if !StdinIsTerminal() {
		return nil, nil, nil
	}
	rl, err := readline.NewEx(&readline.Config{
		Prompt:       defaultPrompt,
		Stdin:        multiline.NewMultilineStdin(multiline.PlatformStdin()),
		AutoComplete: replcomplete.NewReplCompleter(replcomplete.ReplCompleteEnv{}),
	})
	if err != nil {
		return nil, nil, err
	}
	fn := func(prompt string) (string, error) {
		return Prompt(rl, prompt)
	}
	return rl, fn, nil
}
