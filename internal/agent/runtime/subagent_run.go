package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/turnloop"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/title"
)

func (r *Runtime) nestedActive() bool {
	r.nestedMu.Lock()
	defer r.nestedMu.Unlock()
	return r.nestedState != nil
}

func (r *Runtime) setNestedState(st *activeNestedState) {
	r.nestedMu.Lock()
	r.nestedState = st
	r.nestedMu.Unlock()
}

func (r *Runtime) getNestedState() *activeNestedState {
	r.nestedMu.Lock()
	defer r.nestedMu.Unlock()
	return r.nestedState
}

func (r *Runtime) runSubagentTool(ctx context.Context, cfg NestedRunConfig) (NestedRunResult, error) {
	if r.nestedActive() {
		return r.delegateSubagentFromNested(ctx, cfg)
	}
	if cfg.Interrupt {
		if cfg.ResumeID == "" {
			return NestedRunResult{}, fmt.Errorf("interrupt requires resume")
		}
		if err := globalSubagentRegistry.cancelAndWait(cfg.ResumeID, 10*time.Second); err != nil {
			return NestedRunResult{}, err
		}
	}
	if cfg.ResumeID != "" && globalSubagentRegistry.isRunning(cfg.ResumeID) {
		return NestedRunResult{}, fmt.Errorf("subchat %s is running; use /subagent stop first", cfg.ResumeID)
	}
	if cfg.RunInBackground {
		return r.startSubagentBackground(ctx, cfg)
	}
	return r.runNestedWithConfig(ctx, cfg)
}

func (r *Runtime) startSubagentBackground(ctx context.Context, cfg NestedRunConfig) (NestedRunResult, error) {
	if err := r.resolveSubagentRole(&cfg); err != nil {
		return NestedRunResult{}, err
	}
	sess, id, err := r.prepareSubSession(ctx, cfg)
	if err != nil {
		return NestedRunResult{}, err
	}
	if strings.TrimSpace(cfg.RoleModel) == "" && sess != nil {
		cfg.RoleProvider, cfg.RoleModel = subagentRoleFromSession(sess)
	}
	if strings.TrimSpace(cfg.RoleProvider) != "" && strings.TrimSpace(cfg.RoleModel) != "" {
		if err := r.resolveSubagentRole(&cfg); err != nil {
			return NestedRunResult{}, err
		}
	}
	if err := r.persistSubSession(sess); err != nil {
		return NestedRunResult{}, err
	}
	bgCtx, cancel := context.WithCancel(context.Background())
	h := globalSubagentRegistry.registerRun(id, cancel)
	title := sess.Title
	_ = globalSubagentRegistry.upsertActiveEntry(r.activeEntryFor(sess))
	go func() {
		defer globalSubagentRegistry.finishRun(id)
		defer cancel()
		res, err := r.runNestedWithConfig(bgCtx, cfgWithID(cfg, sess))
		if !r.machineMode() {
			if err != nil {
				turnloop.WriteSystemDeferred(r.Out, subagentFailedSystemMessage(title, err))
			} else if res.Status == chatstore.SubStatusDone {
				turnloop.WriteSystemDeferred(r.Out, subagentDoneSystemMessage(title, res.SubchatID))
			}
		}
	}()
	_ = h
	return NestedRunResult{SubchatID: id, Status: chatstore.SubStatusRunning, Output: ""}, nil
}

func cfgWithID(cfg NestedRunConfig, sess *chatstore.SubSession) NestedRunConfig {
	cfg.ResumeID = sess.ID
	cfg.Origin = sess.Origin
	cfg.ProjectHex = sess.ProjectHex
	cfg.RunInBackground = true
	return cfg
}

func (r *Runtime) prepareSubSession(ctx context.Context, cfg NestedRunConfig) (*chatstore.SubSession, string, error) {
	if r.EphemeralSession {
		return nil, "", fmt.Errorf("subagent persistence disabled for ephemeral parent session")
	}
	spawn := cfg.SpawnTime
	if spawn.IsZero() {
		spawn = time.Now().UTC()
	}
	parentChatID := cfg.ParentChatID
	if parentChatID == "" && r.Session != nil {
		parentChatID = r.Session.ID
	}
	if cfg.ResumeID != "" {
		sess, err := chatstore.FindSubSessionByID(r.ProjHex, cfg.ResumeID)
		if err != nil {
			return nil, "", err
		}
		if cfg.Task != "" {
			sess.Messages = append(sess.Messages, chatstore.Message{Role: "user", Content: cfg.Task})
			sess.LastMessageAt = time.Now().UTC()
		}
		if cfg.SysPromptPath == "" {
			cfg.SysPromptPath = sess.SysPromptPath
		}
		if cfg.RoleProvider == "" && cfg.RoleModel == "" {
			cfg.RoleProvider, cfg.RoleModel = subagentRoleFromSession(sess)
		}
		sess.Status = chatstore.SubStatusRunning
		return sess, sess.ID, nil
	}
	if strings.TrimSpace(cfg.Task) == "" {
		return nil, "", fmt.Errorf("task is required for new subagent")
	}
	id := chatstore.SubchatID(parentChatID, cfg.ToolCall, spawn)
	titleBackend := r.Backend
	titleModel := r.Model
	if strings.TrimSpace(cfg.RoleModel) != "" {
		titleModel = cfg.RoleModel
		if b, err := r.backendForProvider(ctx, cfg.RoleProvider); err == nil {
			titleBackend = b
		}
	}
	var t string
	if cfg.RunInBackground {
		t = title.FallbackFromWords(cfg.Task)
	} else {
		t, _ = title.FromPrompt(ctx, titleBackend, r.Client, r.Cfg, titleModel, cfg.Task)
	}
	if strings.TrimSpace(t) == "" {
		t = title.FallbackFromWords(cfg.Task)
	}
	t = title.NormalizeSlug(t)
	origin := cfg.Origin
	if origin == "" {
		origin = chatstore.SubOriginParent
	}
	projHex := cfg.ProjectHex
	if projHex == "" {
		projHex = r.ProjHex
	}
	effort, _ := r.Cfg.EffectiveSubagentReasoningEffort(cfg.ReasoningEffort)
	sess := &chatstore.SubSession{
		ID:               id,
		Title:            t,
		CreatedAt:        spawn,
		LastMessageAt:    spawn,
		Origin:           origin,
		ProjectHex:       projHex,
		ParentChatID:     parentChatID,
		ParentToolCallID: cfg.ParentToolCallID,
		SysPromptPath:    cfg.SysPromptPath,
		Status:           chatstore.SubStatusRunning,
		ReasoningEffort:  effort,
		RoleProvider:     strings.TrimSpace(cfg.RoleProvider),
		RoleModel:        strings.TrimSpace(cfg.RoleModel),
		Messages:         []chatstore.Message{{Role: "user", Content: cfg.Task}},
		ImageFiles:       cloneImageFiles(r.Session),
	}
	return sess, id, nil
}

func cloneImageFiles(s *chatstore.Session) map[int]string {
	if s == nil || len(s.ImageFiles) == 0 {
		return nil
	}
	out := make(map[int]string, len(s.ImageFiles))
	for k, v := range s.ImageFiles {
		out[k] = v
	}
	return out
}

func (r *Runtime) persistSubSession(sess *chatstore.SubSession) error {
	if sess == nil || r.EphemeralSession {
		return nil
	}
	return chatstore.WriteSubSession(sess.ProjectHex, sess)
}

func (r *Runtime) activeEntryFor(sess *chatstore.SubSession) chatstore.ActiveSubagentEntry {
	p, _ := chatstore.SubSessionPath(sess.ProjectHex, sess.Origin, sess.ID)
	return chatstore.ActiveSubagentEntry{
		ID:          sess.ID,
		Origin:      sess.Origin,
		Status:      sess.Status,
		SessionPath: p,
		ProjectHex:  sess.ProjectHex,
		SpawnedAt:   sess.CreatedAt,
	}
}

func (r *Runtime) subSessionImageFiles(sess *chatstore.SubSession) map[int]string {
	if sess != nil && len(sess.ImageFiles) > 0 {
		return sess.ImageFiles
	}
	if r.Session != nil {
		return r.Session.ImageFiles
	}
	return nil
}

func (r *Runtime) appendPendingSpawnToParent(p chatstore.PendingSubagentSpawn) {
	if r.Session == nil {
		return
	}
	r.mutateSession(func(s *chatstore.Session) {
		s.Messages = append(s.Messages, chatstore.Message{
			Role:    "user",
			Content: fmt.Sprintf("[subagent-pending] %s", mustJSON(p)),
		})
	})
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func subagentDoneSystemMessage(title, subchatID string) string {
	title = strings.TrimSpace(title)
	subchatID = strings.TrimSpace(subchatID)
	return fmt.Sprintf("subagent %s done\n\t%s", title, subchatID)
}

func subagentFailedSystemMessage(title string, err error) string {
	title = strings.TrimSpace(title)
	if err == nil {
		return fmt.Sprintf("subagent %s failed", title)
	}
	return fmt.Sprintf("subagent %s failed\n\t%v", title, err)
}

func SubagentDoneSystemMessageForTest(title, subchatID string) string {
	return subagentDoneSystemMessage(title, subchatID)
}
