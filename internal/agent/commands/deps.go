package commands

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/llm"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"

	"github.com/openai/openai-go/v2"
)

type Deps struct {
	Ctx context.Context

	Out      io.Writer
	Stdin    io.Reader
	ReadLine func(prompt string) (string, error)

	Cfg     *config.Root
	SaveCfg func() error

	ProjHex  string
	ProjRoot string

	Session    func() *chatstore.Session
	SetSession func(*chatstore.Session)

	SetMode func(string)
	GetMode func() string

	ApplyCurrentModel            func(providerName, modelID string) error
	Model                        func() string
	Provider                     func() *config.Provider
	CompactionThresholdTokens    func() int64
	SetCompactionThresholdTokens func(int64)

	Client  openai.Client
	Backend llm.CompletionBackend

	ResetReadlineHistory  func()
	AppendReadlineHistory func(line string) error

	PrefillInput             func(string)
	SubmitUserMessage        func(string) error
	SubmitVisibleUserMessage func(visible, api string) error

	PrintWelcomeBanner func()

	CheckpointGoto func(*checkpoint.FullCheckpointID) error

	PersistSession func() error
	MutateSession  func(fn func(*chatstore.Session))

	GetReplShellFirst func() bool
	SetReplShellFirst func(bool)

	GetEphemeralSession func() bool
	SetEphemeralSession func(bool)
}

func PrintSystem(out io.Writer, msg string) {
	if out == nil {
		out = os.Stdout
	}
	termcolor.WriteSystem(out, msg)
}

func PrintSystemf(out io.Writer, format string, args ...any) {
	PrintSystem(out, fmt.Sprintf(format, args...))
}

func PrintSystemValue(out io.Writer, v any) {
	PrintSystem(out, termcolor.SystemMessageText(v))
}

func PrintSystemErr(out io.Writer, err error) {
	if err == nil {
		return
	}
	PrintSystem(out, llm.UserFacingAPIError(err))
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
