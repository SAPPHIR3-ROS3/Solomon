package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func (r *Runtime) delegateSubagentFromNested(ctx context.Context, cfg NestedRunConfig) (NestedRunResult, error) {
	st := r.getNestedState()
	if st == nil {
		return NestedRunResult{}, fmt.Errorf("nested subagent state missing")
	}
	parentAvail := r.Session != nil && !r.EphemeralSession && strings.TrimSpace(st.parentChatID) != ""
	if parentAvail {
		prev := r.getNestedState()
		r.setNestedState(nil)
		defer r.setNestedState(prev)
		res, err := r.runSubagentTool(ctx, cfg)
		return res, err
	}
	pending := chatstore.PendingSubagentSpawn{
		RequesterSubchatID: st.subchatID,
		RequesterOrigin:    st.origin,
		ParentChatID:       st.parentChatID,
		ProjectHex:         st.projectHex,
		SysPromptPath:      cfg.SysPromptPath,
		Task:               cfg.Task,
		Resume:             cfg.ResumeID,
		Interrupt:          cfg.Interrupt,
		RunInBackground:    cfg.RunInBackground,
		ReasoningEffort:    cfg.ReasoningEffort,
		RoleProvider:       strings.TrimSpace(cfg.RoleProvider),
		RoleModel:          strings.TrimSpace(cfg.RoleModel),
		ToolCall:           cfg.ToolCall,
		SpawnISO:           cfg.SpawnTime.UTC().Format(time.RFC3339Nano),
		NotifyNewChat:      st.origin == chatstore.SubOriginScheduled,
		CreatedAt:          time.Now().UTC(),
	}
	if sess, err := chatstore.FindSubSessionByID(st.projectHex, st.subchatID); err == nil && sess != nil {
		sess.PendingSpawns = append(sess.PendingSpawns, pending)
		_ = chatstore.WriteSubSession(sess.ProjectHex, sess)
	}
	if hasActiveUserSession() {
		msg := "Return to the parent chat to resolve a pending subagent spawn."
		if pending.NotifyNewChat {
			msg = "Open or create a new chat to resolve a pending subagent spawn from a scheduled subagent."
		}
		termcolor.WriteSystem(r.Out, msg)
	}
	return NestedRunResult{}, fmt.Errorf("parent unavailable for nested subagent spawn")
}

func (r *Runtime) consumePendingSubagentSpawns() {
	if r.Session == nil {
		return
	}
	for _, m := range r.Session.Messages {
		if m.Role != "user" || !strings.HasPrefix(m.Content, "[subagent-pending] ") {
			continue
		}
		raw := strings.TrimPrefix(m.Content, "[subagent-pending] ")
		var p chatstore.PendingSubagentSpawn
		if json.Unmarshal([]byte(raw), &p) != nil {
			continue
		}
		spawn, _ := time.Parse(time.RFC3339Nano, p.SpawnISO)
		cfg := NestedRunConfig{
			SysPromptPath:    p.SysPromptPath,
			Task:             p.Task,
			ResumeID:         p.Resume,
			Interrupt:        p.Interrupt,
			RunInBackground:  p.RunInBackground,
			ReasoningEffort:  p.ReasoningEffort,
			RoleProvider:     strings.TrimSpace(p.RoleProvider),
			RoleModel:        strings.TrimSpace(p.RoleModel),
			ParentChatID:     r.Session.ID,
			ParentToolCallID: p.ToolCall.ID,
			ToolCall:         p.ToolCall,
			SpawnTime:        spawn,
			Origin:           chatstore.SubOriginParent,
			ProjectHex:       r.ProjHex,
		}
		_, _ = r.runSubagentTool(context.Background(), cfg)
	}
}

func (r *Runtime) execToolNestedAware(ctx context.Context, inv tooling.Invocation) (any, error) {
	if inv.Name == "subagent" && r.nestedActive() {
		var args map[string]json.RawMessage
		if err := json.Unmarshal(inv.Args, &args); err != nil {
			return nil, err
		}
		cfg, err := nestedRunConfigFromToolArgs(r, args)
		if err != nil {
			return nil, err
		}
		res, err := r.delegateSubagentFromNested(ctx, cfg)
		if err != nil {
			return map[string]any{"error": err.Error()}, nil
		}
		return map[string]any{"ok": true, "delegated": true, "subchatId": res.SubchatID, "status": res.Status, "output": res.Output}, nil
	}
	return r.execTool(ctx, inv)
}
