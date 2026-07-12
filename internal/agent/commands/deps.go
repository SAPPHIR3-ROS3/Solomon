package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/research"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"

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

	AcquireSessionFileLock func() error
	ReleaseSessionFileLock func()

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
	AtIncludeNotifier        func() *atmention.Notifier

	PrintWelcomeBanner func()

	CheckForUpdate func(force bool) (*updater.Notice, error)
	InstallUpdate    func(tag string) error

	CheckpointGoto   func(*checkpoint.FullCheckpointID) error
	CheckpointRewind func(*checkpoint.RewindPlan) error

	PersistSession func() error
	MutateSession  func(fn func(*chatstore.Session))

	GetReplShellFirst func() bool
	SetReplShellFirst func(bool)

	GetEphemeralSession func() bool
	SetEphemeralSession func(bool)

	Research interface {
		StartResearchJob(query, category string) (research.JobRecord, error)
		ListResearch() ([]research.JobRecord, error)
		ResearchStatus(target string) (research.JobRecord, error)
		CancelResearch(target string) error
		DeleteResearch(target string) error
		ResumeResearch(target string) (research.JobRecord, error)
	}
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

func FormatProviderCatalogError(providerName string, err error) string {
	detail := llm.UserFacingAPIError(err)
	return fmt.Sprintf("provider %s:\n%s", providerName, detail)
}

const providerCatalogErrorSep = "======"

func PrintProviderCatalogErrors(out io.Writer, errs []ProviderCatalogError) {
	if out == nil || len(errs) == 0 {
		return
	}
	parts := make([]string, len(errs))
	for i, e := range errs {
		parts[i] = FormatProviderCatalogError(e.label(), e.Err)
	}
	PrintSystem(out, strings.Join(parts, "\n"+providerCatalogErrorSep+"\n"))
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
