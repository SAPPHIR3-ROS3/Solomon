package agentruntime

import (
	"context"
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/turnloop"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/research"
)

func (r *Runtime) StartResearchJob(query, category string) (research.JobRecord, error) {
	return r.startResearchJob(context.Background(), query, category)
}

func (r *Runtime) startResearchJob(ctx context.Context, query, category string) (research.JobRecord, error) {
	if r.EphemeralSession {
		return research.JobRecord{}, fmt.Errorf("research persistence disabled for ephemeral parent session")
	}
	parentChatID := ""
	if r.Session != nil {
		parentChatID = r.Session.ID
	}
	out := r.Out
	return research.GlobalManager().Start(ctx, research.StartRequest{
		Question:     query,
		Category:     category,
		ProjectHex:   r.ProjHex,
		ParentChatID: parentChatID,
		Model:        r.Model,
		Cfg:          r.Cfg,
		Backend:      r.Backend,
		OnProgress: func(rec research.JobRecord, ev research.ProgressEvent) {
			if r.machineMode() {
				return
			}
			turnloop.WriteSystemDeferred(out, research.FormatProgressLine(rec, ev))
		},
		OnDone: func(rec research.JobRecord) {
			if r.machineMode() {
				return
			}
			switch rec.Status {
			case research.StatusDone:
				turnloop.WriteSystemDeferred(out, research.FormatDoneMessage(rec))
			case research.StatusFailed:
				turnloop.WriteSystemDeferred(out, fmt.Sprintf("research %s failed\n\t%s", rec.Title, rec.Error))
			case research.StatusCancelled:
				turnloop.WriteSystemDeferred(out, fmt.Sprintf("research %s cancelled", rec.Title))
			case research.StatusPaused:
				turnloop.WriteSystemDeferred(out, research.FormatPausedMessage(rec))
			}
		},
	})
}

func (r *Runtime) ResearchStatus(target string) (research.JobRecord, error) {
	return r.ResearchStatusForProject(r.ProjHex, target)
}

func (r *Runtime) ResearchStatusForProject(projectHex, target string) (research.JobRecord, error) {
	return research.GlobalManager().Get(projectHex, target)
}

func (r *Runtime) CancelResearch(target string) error {
	return research.GlobalManager().Cancel(r.ProjHex, target)
}

func (r *Runtime) DeleteResearch(target string) error {
	return research.GlobalManager().Delete(r.ProjHex, target)
}

func (r *Runtime) ResumeResearch(target string) (research.JobRecord, error) {
	if r.EphemeralSession {
		return research.JobRecord{}, fmt.Errorf("research persistence disabled for ephemeral parent session")
	}
	out := r.Out
	return research.GlobalManager().Resume(context.Background(), r.ProjHex, target, research.StartRequest{
		ProjectHex: r.ProjHex,
		Model:      r.Model,
		Cfg:        r.Cfg,
		Backend:    r.Backend,
		OnProgress: func(rec research.JobRecord, ev research.ProgressEvent) {
			if r.machineMode() {
				return
			}
			turnloop.WriteSystemDeferred(out, research.FormatProgressLine(rec, ev))
		},
		OnDone: func(rec research.JobRecord) {
			if r.machineMode() {
				return
			}
			switch rec.Status {
			case research.StatusDone:
				turnloop.WriteSystemDeferred(out, research.FormatDoneMessage(rec))
			case research.StatusFailed:
				turnloop.WriteSystemDeferred(out, fmt.Sprintf("research %s failed\n\t%s", rec.Title, rec.Error))
			case research.StatusCancelled:
				turnloop.WriteSystemDeferred(out, fmt.Sprintf("research %s cancelled", rec.Title))
			case research.StatusPaused:
				turnloop.WriteSystemDeferred(out, research.FormatPausedMessage(rec))
			}
		},
	})
}

func (r *Runtime) ListResearch() ([]research.JobRecord, error) {
	return research.GlobalManager().List(r.ProjHex)
}
