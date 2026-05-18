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

	Out io.Writer

	MCP *solomonmcp.Manager

	ReplShellFirst bool
}

func NewRuntime(rl *readline.Instance, cfg *config.Root, prov *config.Provider, projHex, projRoot string, sess *chatstore.Session) *Runtime {
	cl := openai.NewClient(
		option.WithAPIKey(prov.APIKey),
		option.WithBaseURL(prov.BaseURL),
	)
	return &Runtime{
		RL:                        rl,
		Client:                    cl,
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
	r.RL.SetPrompt(checkpoint.FormatReplPromptPrefix(r.Session) + termcolor.WrapUser("You: "))
}

func (r *Runtime) refreshReadlinePromptContinue() {
	if r.RL == nil {
		return
	}
	r.RL.SetPrompt(checkpoint.FormatReplPromptPrefix(r.Session) + termcolor.WrapUser(".... "))
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
	return r.onUserMessage(ctx, trimMessageEdges(clean), false)
}

func (r *Runtime) persistSession() error {
	if r.EphemeralSession {
		return nil
	}
	r.chatPersistMu.Lock()
	defer r.chatPersistMu.Unlock()
	return chatstore.WriteSession(r.ProjHex, r.Session)
}

func (r *Runtime) persistSessionUnsafe() error {
	return chatstore.WriteSession(r.ProjHex, r.Session)
}
