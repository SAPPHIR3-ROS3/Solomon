package connect

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

type Deps struct {
	Ctx context.Context

	Out      io.Writer
	Stdin    io.Reader
	ReadLine func(prompt string) (string, error)

	Cfg     *config.Root
	SaveCfg func() error

	ApplyCurrentModel func(providerName, modelID string) error
	Model             func() string
	Provider          func() *config.Provider
}

func PromptIO(d Deps) config.PromptIO {
	stdin := d.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	out := d.Out
	if out == nil {
		out = os.Stdout
	}
	return config.PromptIO{Stdin: stdin, Out: out, ReadLine: d.ReadLine}
}

func printSystem(d Deps, msg string) {
	out := d.Out
	if out == nil {
		out = os.Stdout
	}
	termcolor.WriteSystem(out, msg)
}

func printSystemf(d Deps, format string, args ...any) {
	printSystem(d, fmt.Sprintf(format, args...))
}
