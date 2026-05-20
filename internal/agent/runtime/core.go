package agentruntime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/agent/cievents"
	agenttools "github.com/SAPPHIR3-ROS3/Solomon/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/internal/mcp"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/prompt"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"

	readline "github.com/chzyer/readline"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

var errUserStopGeneration = errors.New("user stopped generation")

const cliMsgGenerationStopped = "Generation stopped"

type Runtime struct {
	RL *readline.Instance

	Client openai.Client
	Model  string
	Cfg    *config.Root
	Prov   *config.Provider

	ProjHex  string
	ProjRoot string

	Mode string

	Session *chatstore.Session

	CompactionThresholdTokens int64

	EphemeralSession bool

	chatPersistMu              sync.Mutex
	deferredTitleScheduleMu    sync.Mutex
	deferredTitleWorkerRunning bool
	sessionFileCreated         bool

	Out io.Writer

	MCP *solomonmcp.Manager

	ReplShellFirst bool

	EventSink       cievents.Sink
	FailOnToolError bool

	ciPrompt        string
	ciTurn          int
	ciToolErr       bool
	ciFinalContent  string
}

func NewRuntime(rl *readline.Instance, cfg *config.Root, prov *config.Provider, projHex, projRoot string, sess *chatstore.Session) *Runtime {
	rt := &Runtime{
		RL:                        rl,
		Model:                     cfg.Current.Model,
		Cfg:                       cfg,
		Prov:                      prov,
		ProjHex:                   projHex,
		ProjRoot:                  projRoot,
		Mode:                      "build",
		Session:                   sess,
		CompactionThresholdTokens: config.EffectiveCompactionThresholdTokens(cfg),
		Out:                       os.Stdout,
	}
	if prov != nil {
		rt.Client = openai.NewClient(
			option.WithAPIKey(prov.APIKey),
			option.WithBaseURL(prov.BaseURL),
		)
	}
	return rt
}

func (r *Runtime) ApplyCurrentModel(providerName, modelID string) error {
	prevP, prevM := r.Cfg.Current.Provider, r.Cfg.Current.Model
	changed := prevP != providerName || prevM != modelID
	r.Cfg.Current.Provider = providerName
	r.Cfg.Current.Model = modelID
	if changed {
		config.NoteRecentModelUse(r.Cfg, providerName, modelID)
	}
	if err := config.Save(r.Cfg); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "save config failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return err
	}
	for i := range r.Cfg.Providers {
		if r.Cfg.Providers[i].Name == providerName {
			p := &r.Cfg.Providers[i]
			r.Prov = p
			r.Model = modelID
			r.Client = openai.NewClient(
				option.WithAPIKey(p.APIKey),
				option.WithBaseURL(p.BaseURL),
			)
			return nil
		}
	}
	return fmt.Errorf("provider %q not found", providerName)
}

func (r *Runtime) refreshReadlinePrompt() {
	if r.RL == nil {
		return
	}
	var prefix string
	r.mutateSession(func(s *chatstore.Session) {
		prefix = checkpoint.FormatReplPromptPrefix(s) + termcolor.WrapUser("You: ")
	})
	r.RL.SetPrompt(prefix)
}

func (r *Runtime) refreshReadlinePromptContinue() {
	if r.RL == nil {
		return
	}
	var prefix string
	r.mutateSession(func(s *chatstore.Session) {
		prefix = checkpoint.FormatReplPromptPrefix(s) + termcolor.WrapUser(".... ")
	})
	r.RL.SetPrompt(prefix)
}

func (r *Runtime) systemPrompt(disableThinking bool) (string, error) {
	var dump string
	var err error
	if r.Mode == "plan" {
		dump, err = agenttools.BuildPlanToolDump()
	} else {
		dump, err = agenttools.BuildBuildToolDump()
	}
	if err != nil {
		return "", err
	}
	if r.MCP != nil {
		if mcpDump := strings.TrimSpace(r.MCP.ToolDump()); mcpDump != "" {
			dump = strings.TrimSpace(dump + "\n---\n" + mcpDump)
		}
	}
	absWorkspace := r.ProjRoot
	if p, err := filepath.Abs(r.ProjRoot); err == nil {
		absWorkspace = p
	}
	syntax := prompt.NativeToolInvocationSyntax()
	if r.Session.LegacyTools {
		syntax = strings.TrimSpace(syntax + "\n\n" + prompt.LegacyToolInvocationSyntaxAppend())
	}
	d := prompt.Data{
		Tools:                 dump,
		Syntax:                syntax,
		ExtraRules:            "",
		Language:              r.Cfg.EffectiveResponseLanguage(),
		UserName:              strings.TrimSpace(r.Cfg.UserName),
		DisableThinking:       disableThinking,
		WorkspaceAbsolutePath: absWorkspace,
	}
	if r.Mode == "plan" {
		return prompt.RenderPlan(d)
	}
	return prompt.RenderBuild(d)
}

func (r *Runtime) RunPromptOnce(ctx context.Context, line string) error {
	clean, _ := parseMultilineControlRunes(line)
	line = trimMessageEdges(clean)
	if r.machineMode() {
		return r.runPromptOnceCI(ctx, line)
	}
	return r.onUserMessage(ctx, line, false)
}

func (r *Runtime) mutateSession(fn func(*chatstore.Session)) {
	r.chatPersistMu.Lock()
	defer r.chatPersistMu.Unlock()
	fn(r.Session)
}

func (r *Runtime) markSessionFileCreated() {
	r.sessionFileCreated = true
}

func (r *Runtime) shouldWriteSessionFile() bool {
	if r.EphemeralSession || !r.sessionFileCreated {
		return false
	}
	return r.Session != nil && r.Session.ID != ""
}

func (r *Runtime) writeSessionLocked() error {
	if !r.shouldWriteSessionFile() {
		return nil
	}
	return chatstore.WriteSession(r.ProjHex, r.Session)
}

func (r *Runtime) persistSession() error {
	r.chatPersistMu.Lock()
	defer r.chatPersistMu.Unlock()
	return r.writeSessionLocked()
}

func (r *Runtime) persistSessionUnsafe() error {
	return r.writeSessionLocked()
}

func (r *Runtime) sessionMessagesSnapshot() (msgs []chatstore.Message, imageFiles map[int]string) {
	r.chatPersistMu.Lock()
	defer r.chatPersistMu.Unlock()
	msgs = append([]chatstore.Message(nil), r.Session.Messages...)
	if len(r.Session.ImageFiles) == 0 {
		return msgs, nil
	}
	imageFiles = make(map[int]string, len(r.Session.ImageFiles))
	for k, v := range r.Session.ImageFiles {
		imageFiles[k] = v
	}
	return msgs, imageFiles
}
