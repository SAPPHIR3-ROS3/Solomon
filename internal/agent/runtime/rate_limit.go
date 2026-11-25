package agentruntime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func (r *Runtime) ResumeRateLimitedRun(ctx context.Context, agents []*chatstore.SubSession, resumeParent bool) error {
	r.markSessionFileCreated()
	return r.runCIWithPrompt("", func() error {
		if err := r.waitProviderReady(ctx); err != nil {
			return err
		}
		if err := r.resumeRateLimitedAgents(ctx, agents); err != nil {
			return err
		}
		if !resumeParent {
			return nil
		}
		defer r.saveTurnFileChanges(r.Session.CheckpointLast)
		return r.runAgentTurns(ctx)
	})
}

func (r *Runtime) resumeRateLimitedAgents(ctx context.Context, agents []*chatstore.SubSession) error {
	sort.SliceStable(agents, func(i, j int) bool {
		return subagentSessionDepth(agents[i]) > subagentSessionDepth(agents[j])
	})
	for len(agents) > 0 {
		n := 1
		for n < len(agents) && subagentSessionDepth(agents[n]) == subagentSessionDepth(agents[0]) {
			n++
		}
		outputs, failures := make([]string, n), make([]error, n)
		var group sync.WaitGroup
		for i, agent := range agents[:n] {
			group.Go(func() {
				outputs[i], failures[i] = r.resumeRateLimitedAgent(ctx, agent)
			})
		}
		group.Wait()
		for i, agent := range agents[:n] {
			if failures[i] == nil {
				failures[i] = r.deliverResumedAgentResult(agent, outputs[i])
			}
		}
		if err := errors.Join(failures...); err != nil {
			return err
		}
		agents = agents[n:]
	}
	return nil
}

func (r *Runtime) resumeRateLimitedAgent(ctx context.Context, agent *chatstore.SubSession) (string, error) {
	if globalSubagentRegistry.isRunning(agent.ID) {
		return "", fmt.Errorf("subagent %s is already running", agent.ID)
	}
	provider, model := subagentRoleFromSession(agent)
	if provider == "" && model == "" {
		provider, model = agent.Provider, agent.Model
	}
	for i := len(agent.Messages) - 1; provider == "" && model == "" && i >= 0; i-- {
		msg := agent.Messages[i]
		if msg.Provider != "" && msg.Model != "" {
			provider, model = msg.Provider, msg.Model
		}
	}
	if provider == "" && model == "" {
		provider, model = r.Prov.Name, r.Model
	}
	result, err := r.runNestedWithConfig(ctx, NestedRunConfig{
		savedTarget: true, ResumeID: agent.ID, Depth: subagentSessionDepth(agent),
		ParentToolCallID: agent.ParentToolCallID, ParentChatID: agent.ParentChatID,
		ProjectHex: agent.ProjectHex, Origin: agent.Origin,
		RoleProvider: provider, RoleModel: model, ReasoningEffort: agent.ReasoningEffort,
	})
	return result.Output, err
}

func (r *Runtime) deliverResumedAgentResult(agent *chatstore.SubSession, output string) error {
	if text := []rune(output); len(text) > 16000 {
		output = string(text[len(text)-16000:])
	}
	message := chatstore.Message{
		Role: "user", GUIHidden: true, CreatedAt: time.Now(),
		Content: fmt.Sprintf("[subagent-completed] Subagent %s (subchat %s) resumed and finished.\nResult:\n%s", agent.Title, agent.ID, output),
	}
	if agent.ParentSubchatID == "" {
		r.mutateSession(func(session *chatstore.Session) { session.Messages = append(session.Messages, message) })
		return r.persistSession()
	}
	parent, err := chatstore.FindSubSessionByID(r.ProjHex, agent.ParentSubchatID)
	if err != nil {
		return err
	}
	parent.Messages = append(parent.Messages, message)
	return r.persistSubSession(parent)
}
