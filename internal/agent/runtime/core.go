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
	"github.com/SAPPHIR3-ROS3/Solomon/internal/instructions"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/internal/mcp"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/prompt"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooloutput"

	readline "github.com/chzyer/readline"
	"github.com/openai/openai-go/v2"
)

var errUserStopGeneration = errors.New("user stopped generation")

const cliMsgGenerationStopped = "Generation stopped"

type Runtime struct {
	RL *readline.Instance

	Client  openai.Client
	Backend llm.CompletionBackend
	Model   string
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

	ToolOut *tooloutput.Service

	Instructions *instructions.Loader
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
		ToolOut:                   tooloutput.NewService(projHex, tooloutput.LimitsFromConfig(cfg)),
		Instructions:              instructions.NewLoader(),
	}
	if err := tooloutput.Startup(os.Getpid()); err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "tool output instance register failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
	}
	if prov != nil {
		rt.applyProviderClient(context.Background(), prov)
	}
	return rt
}

func (r *Runtime) currentSessionID() string {
	r.chatPersistMu.Lock()
	defer r.chatPersistMu.Unlock()
	if r.Session == nil {
		return ""
	}
	return r.Session.ID
}

func (r *Runtime) applyToolOutput(res any, toolName, toolCallID string) any {
	if r == nil || r.ToolOut == nil {
		return res
	}
	return r.ToolOut.Apply(res, tooloutput.Meta{
		SessionID:  r.currentSessionID(),
		ToolCallID: toolCallID,
		ToolName:   toolName,
	})
}

func (r *Runtime) applyProviderClient(ctx context.Context, p *config.Provider) {
	backend, err := llm.NewCompletionBackend(ctx, r.Cfg, p)
	if err != nil {
		params := map[string]any{"err": err.Error()}
		if p != nil {
			params["provider"] = p.Name
		}
		logging.Log(logging.ERROR_LOG_LEVEL, "apply provider client failed", logging.LogOptions{Params: params})
		return
	}
	r.Backend = backend
	if c, ok := llm.OpenAIClientFromBackend(backend); ok {
		r.Client = c
	} else {
		r.Client = openai.Client{}
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
	if p := config.ProviderByName(r.Cfg, providerName); p != nil {
		r.Prov = p
		r.Model = modelID
		backend, err := llm.NewCompletionBackend(context.Background(), r.Cfg, p)
		if err != nil {
			return err
		}
		r.Backend = backend
		if c, ok := llm.OpenAIClientFromBackend(backend); ok {
			r.Client = c
		} else {
			r.Client = openai.Client{}
		}
		return nil
	}
	return fmt.Errorf("provider %q not found", providerName)
}

func (r *Runtime) refreshReadlinePrompt() {
	if r.RL == nil {
		return
	}
	var prefix string
	r.mutateSession(func(s *chatstore.Session) {
		prefix = checkpoint.FormatReplPromptPrefix(s) + termcolor.WrapUserReadline("You: ")
	})
	r.RL.SetPrompt(prefix)
}

func (r *Runtime) refreshReadlinePromptContinue() {
	if r.RL == nil {
		return
	}
	var prefix string
	r.mutateSession(func(s *chatstore.Session) {
		prefix = checkpoint.FormatReplPromptPrefix(s) + termcolor.WrapUserReadline(".... ")
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
	legacyEnabled := r.legacyToolsEnabled()
	legacyForced := r.legacyToolsForced()
	var syntax string
	var legacySyntax string
	if legacyForced {
		syntax = prompt.LegacyOnlyToolInvocationSyntax()
	} else {
		syntax = prompt.NativeToolInvocationSyntax(legacyEnabled)
		if legacyEnabled {
			legacySyntax = prompt.LegacyToolInvocationSyntaxAppend()
		}
	}
	d := prompt.Data{
		Tools:                 dump,
		Syntax:                syntax,
		LegacySyntax:          legacySyntax,
		LegacyToolsEnabled:    legacyEnabled,
		LegacyToolsForced:     legacyForced,
		Language:              r.Cfg.EffectiveResponseLanguage(),
		UserName:              strings.TrimSpace(r.Cfg.UserName),
		DisableThinking:       disableThinking,
		WorkspaceAbsolutePath: absWorkspace,
	}
	if r.Instructions != nil && r.Session != nil {
		sections, err := r.Instructions.BuildPromptSections(r.ProjRoot, r.ProjHex, r.Session.ActivatedInstructionDirs)
		if err != nil {
			return "", err
		}
		d.CustomRules = sections.CustomRules
		d.GlobalInstructions = sections.GlobalInstructions
		d.RepoInstructions = sections.RepoInstructions
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
