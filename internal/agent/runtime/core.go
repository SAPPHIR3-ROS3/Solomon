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

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/multiline"
	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint/staging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/instructions"
	cursorint "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor"
	cursoragent "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor/agent"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooloutput"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"

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
	Cfg     *config.Root
	Prov    *config.Provider

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

	ciPrompt       string
	ciTurn         int
	ciToolErr      bool
	ciFinalContent string

	ToolOut *tooloutput.Service

	currentToolCpSeq    int
	stagingCache        *staging.Store
	stagingCacheSession string

	Instructions *instructions.Loader

	providerReady chan struct{}

	updateMu       sync.Mutex
	updateChecked  bool
	updateNotice   *updater.Notice
	updateCheckErr error

	pendingUpdateTag string

	nestedMu                  sync.Mutex
	nestedState               *activeNestedState
	currentSubagentToolCallID string
}

func NewRuntime(rl *readline.Instance, cfg *config.Root, prov *config.Provider, projHex, projRoot string, sess *chatstore.Session) *Runtime {
	rt := &Runtime{
		RL:                        rl,
		Model:                     cfg.Current.Model,
		Cfg:                       cfg,
		Prov:                      prov,
		ProjHex:                   projHex,
		ProjRoot:                  projRoot,
		Mode:                      "agent",
		Session:                   sess,
		CompactionThresholdTokens: config.EffectiveCompactionThresholdTokens(cfg),
		Out:                       os.Stdout,
		ToolOut:                   tooloutput.NewService(projHex, tooloutput.LimitsFromConfig(cfg)),
		Instructions:              instructions.NewLoader(),
		providerReady:             make(chan struct{}),
	}
	if err := tooloutput.Startup(os.Getpid()); err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "tool output instance register failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
	}
	globalSubagentRegistry.reconcileOnStartup()
	rt.ensureCursorAPISidecar(context.Background())
	if prov != nil {
		p := prov
		go func() {
			rt.applyProviderClient(context.Background(), p)
			if p.IsCursorAPI() {
				cwd := rt.ProjRoot
				if cwd == "" {
					cwd, _ = os.Getwd()
				}
				if err := cursorint.WaitSidecarIfConfigured(context.Background(), rt.Cfg, cwd, cursoragent.BootstrapIO(rt.Out)); err != nil {
					logging.Log(logging.ERROR_LOG_LEVEL, "cursor API sidecar ensure failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
					termcolor.WriteSystem(rt.Out, "Cursor API sidecar failed to start: "+err.Error())
				}
			}
			close(rt.providerReady)
		}()
	} else {
		close(rt.providerReady)
	}
	return rt
}

func (r *Runtime) waitProviderReady(ctx context.Context) error {
	if r == nil || r.providerReady == nil {
		return nil
	}
	select {
	case <-r.providerReady:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Runtime) ensureCursorAPISidecar(ctx context.Context) {
	if r == nil || r.Cfg == nil {
		return
	}
	cwd := r.ProjRoot
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	cursorint.KickSidecarIfConfigured(ctx, r.Cfg, cwd, cursoragent.BootstrapIO(r.Out))
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
	if prevP != "" {
		if old := config.ProviderByName(r.Cfg, prevP); old != nil && old.IsCursorAPI() && providerName != prevP {
			if np := config.ProviderByName(r.Cfg, providerName); np == nil || !np.IsCursorAPI() {
				r.stripCursorLegacyToolCallsFromSession()
			}
		}
	}
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
	commands.InvalidateAndPrefetchSlashModelCatalog(context.Background(), r.Cfg, r.Out)
	if p := config.ProviderByName(r.Cfg, providerName); p != nil {
		r.Prov = p
		r.Model = modelID
		if p.IsCursorAPI() {
			r.applyProviderClient(context.Background(), p)
			return nil
		}
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

func (r *Runtime) readlinePromptPrimary() string {
	var prefix string
	r.mutateSession(func(s *chatstore.Session) {
		prefix = checkpoint.FormatReplPromptPrefix(s) + termcolor.WrapUserReadline("You: ")
	})
	return prefix
}

func (r *Runtime) readlinePromptContinue() string {
	var prefix string
	r.mutateSession(func(s *chatstore.Session) {
		prefix = checkpoint.FormatReplPromptPrefix(s) + termcolor.WrapUserReadline(".... ")
	})
	return prefix
}

func (r *Runtime) systemPrompt(disableThinking bool) (string, error) {
	legacyForced := r.legacyToolsForcedInPrompt()
	var dump string
	var err error
	switch agenttools.NormalizeMode(r.Mode) {
	case "chat":
		dump, err = agenttools.BuildChatToolDump()
	case "agent":
		dump, err = agenttools.BuildAgentToolDump()
		if err == nil && r.Session != nil && r.Session.PlanningActive {
			planDump, pdErr := agenttools.BuildPlanningNativeToolDump()
			if pdErr != nil {
				err = pdErr
			} else if strings.TrimSpace(planDump) != "" {
				dump = strings.TrimSpace(dump + "\n---\n" + planDump)
			}
		}
		if err == nil && legacyForced {
			defDump, dErr := agenttools.BuildDeferredToolDump()
			if dErr != nil {
				err = dErr
			} else if strings.TrimSpace(defDump) != "" {
				dump = strings.TrimSpace(dump + "\n---\n" + defDump)
			}
		}
	default:
		dump, err = agenttools.BuildAgentToolDump()
	}
	if err != nil {
		return "", err
	}
	if r.MCP != nil && agenttools.NormalizeMode(r.Mode) == "agent" {
		if mcpDump := strings.TrimSpace(r.MCP.ToolDump()); mcpDump != "" {
			section := "MCP tools (native tool_call, names MCP.<server>.<tool>):\n" + mcpDump
			dump = strings.TrimSpace(dump + "\n---\n" + section)
		}
	}
	absWorkspace := r.ProjRoot
	if p, err := filepath.Abs(r.ProjRoot); err == nil {
		absWorkspace = p
	}
	bridge := r.externalToolBridge()
	legacyEnabled := r.legacyToolsEnabledInPrompt()
	planningActive := r.Session != nil && r.Session.PlanningActive
	var syntax string
	var legacySyntax string
	anonymize := r.Cfg != nil && r.Cfg.Anonymize
	if anonymize {
		syntax = prompt.AnonymizeNativeToolInvocationSyntax()
		legacySyntax = ""
	} else if legacyForced {
		syntax = prompt.LegacyOnlyToolInvocationSyntax(planningActive)
	} else if bridge {
		syntax = prompt.NativeToolInvocationSyntax(false)
	} else {
		syntax = prompt.NativeToolInvocationSyntax(legacyEnabled)
		if legacyEnabled {
			legacySyntax = prompt.LegacyToolInvocationSyntaxAppend(planningActive)
		}
	}
	d := prompt.Data{
		Tools:                 dump,
		Syntax:                syntax,
		LegacySyntax:          legacySyntax,
		LegacyToolsEnabled:    legacyEnabled,
		LegacyToolsForced:     legacyForced,
		ExternalToolBridge:    bridge,
		Language:              r.Cfg.EffectiveResponseLanguage(),
		UserName:              strings.TrimSpace(r.Cfg.UserName),
		DisableThinking:        disableThinking,
		WorkspaceAbsolutePath:  absWorkspace,
		Anonymize:              anonymize,
	}
	if r.Session != nil {
		d.PlanningActive = r.Session.PlanningActive
		d.ActivePlanName = r.Session.ActivePlanName
		d.PlanImplementing = r.Session.PlanImplementing
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
	var s string
	switch agenttools.NormalizeMode(r.Mode) {
	case "chat":
		s, err = prompt.RenderChat(d)
	default:
		s, err = prompt.RenderAgent(d)
	}
	if err != nil {
		return "", err
	}
	return chatstore.ScrubLiteralImgPlaceholdersForAPI(s), nil
}

func (r *Runtime) RunPromptOnce(ctx context.Context, line string) error {
	clean, _ := multiline.ParseMultilineControlRunes(line)
	line = multiline.TrimMessageEdges(clean)
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

func (r *Runtime) persistSessionOrLog(context string) {
	if err := r.persistSession(); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "persist session failed", logging.LogOptions{Params: map[string]any{"context": context, "err": err.Error()}})
	}
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
