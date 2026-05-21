package commands

import (
	"context"
	"io"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/llm"

	"github.com/openai/openai-go/v2"
)

type Deps struct {
	Ctx context.Context

	Out   io.Writer
	Stdin io.Reader
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

	PrefillInput      func(string)
	SubmitUserMessage func(string) error

	PrintWelcomeBanner func()

	CheckpointGoto func(*checkpoint.FullCheckpointID) error

	PersistSession func() error
	MutateSession  func(fn func(*chatstore.Session))

	GetReplShellFirst func() bool
	SetReplShellFirst func(bool)

	GetEphemeralSession func() bool
	SetEphemeralSession func(bool)
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
