package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

type rateLimitSchedule struct {
	cancel context.CancelFunc
	at     time.Time
}

func rateLimitedSubagents(projectID string, chat *chatstore.Session) ([]*chatstore.SubSession, error) {
	sessions, err := chatstore.ListSubSessions(projectID)
	if err != nil {
		return nil, err
	}
	messagesByParent := map[string][]chatstore.Message{"": chat.Messages}
	for _, sess := range sessions {
		if sess.ParentChatID == chat.ID && sess.Origin == chatstore.SubOriginParent {
			messagesByParent[sess.ID] = sess.Messages
		}
	}
	var agents []*chatstore.SubSession
	for _, sess := range sessions {
		if sess.ParentChatID != chat.ID || sess.Origin != chatstore.SubOriginParent || sess.Status != chatstore.SubStatusPaused {
			continue
		}
		if sess.RateLimitNotice == "" && chat.RateLimitResume == nil && chat.RateLimitNotice != "" {
			for _, message := range messagesByParent[sess.ParentSubchatID] {
				if message.Role != "tool" || message.ToolCallID == "" || message.ToolCallID != sess.ParentToolCallID {
					continue
				}
				var result struct{ Error string }
				if json.Unmarshal([]byte(message.Content), &result) != nil || result.Error == "" {
					continue
				}
				if notice, _, limited := llm.RateLimitDetails(errors.New(result.Error), time.Now()); limited {
					sess.RateLimitNotice, sess.RateLimitResetAt = notice, llm.RateLimitResetTime(result.Error)
				}
			}
		}
		if sess.RateLimitNotice != "" {
			agents = append(agents, sess)
		}
	}
	return agents, nil
}

func rateLimitTarget(cfg *config.Root, rt *agentruntime.Runtime) chatstore.RateLimitResume {
	fastMode := cfg.EffectiveFastMode()
	return chatstore.RateLimitResume{
		Provider: rt.Prov.Name, Model: rt.Model, ReasoningEffort: cfg.ReasoningEffort, FastMode: &fastMode,
	}
}

func saveRateLimitState(projectID string, sess *chatstore.Session, target chatstore.RateLimitResume, promptErr error) {
	notice, reset, limited := llm.RateLimitDetails(promptErr, time.Now())
	target.ResumeParent = limited
	agents, err := rateLimitedSubagents(projectID, sess)
	if err != nil {
		logging.Diagnostic("unable to inspect rate-limited subagents", map[string]any{"error": err.Error()})
	}
	for _, agent := range agents {
		if notice == "" {
			notice = agent.RateLimitNotice
		}
		if agent.RateLimitResetAt.After(reset) {
			reset = agent.RateLimitResetAt
		}
	}
	var resume *chatstore.RateLimitResume
	if notice != "" {
		target.ResetAt = reset
		resume = &target
	}
	if notice != sess.RateLimitNotice || resume != nil || sess.RateLimitResume != nil {
		sess.RateLimitNotice, sess.RateLimitResume = notice, resume
		if err := chatstore.WriteSession(projectID, sess); err != nil {
			logging.Diagnostic("unable to save rate-limit state", map[string]any{"error": err.Error()})
		}
	}
}

func savedRateLimitTarget(sess *chatstore.Session) (chatstore.RateLimitResume, error) {
	if sess.RateLimitResume != nil {
		return *sess.RateLimitResume, nil
	}
	for i := len(sess.Messages) - 1; i >= 0; i-- {
		msg := sess.Messages[i]
		if msg.Role == "user" && !msg.GUIHidden {
			break
		}
		if msg.Role == "assistant" && msg.Provider != "" && msg.Model != "" {
			return chatstore.RateLimitResume{
				Provider: msg.Provider, Model: msg.Model, ReasoningEffort: msg.ReasoningEffort,
				FastMode: msg.FastMode, ResumeParent: true, ResetAt: llm.RateLimitResetTime(sess.RateLimitNotice),
			}, nil
		}
	}
	return chatstore.RateLimitResume{}, errors.New("the interrupted model was not recorded for this chat")
}

func (a *chatAPI) handleRateLimitSchedule(w http.ResponseWriter, r *http.Request, projectID, root, chatID string) {
	key := projectID + "\x00" + chatID
	if r.Method == http.MethodDelete {
		a.activeMu.Lock()
		a.cancelRateLimitScheduleLocked(key)
		a.activeMu.Unlock()
		w.WriteHeader(http.StatusNoContent)
		return
	}
	sess, err := chatstore.ReadSession(projectID, chatID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err)
		return
	}
	if sess.RateLimitNotice == "" {
		writeAPIError(w, http.StatusConflict, errors.New("chat has no rate-limited run"))
		return
	}
	target, err := savedRateLimitTarget(sess)
	if err != nil {
		writeAPIError(w, http.StatusConflict, err)
		return
	}
	agents, err := rateLimitedSubagents(projectID, sess)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	for _, agent := range agents {
		if agent.RateLimitResetAt.After(target.ResetAt) {
			target.ResetAt = agent.RateLimitResetAt
		}
	}
	surface := clientSurface(r, "")
	a.activeMu.Lock()
	if a.active[key] != nil {
		a.activeMu.Unlock()
		writeAPIError(w, http.StatusConflict, errors.New("chat is already running"))
		return
	}
	schedule := a.rateLimitSchedules[key]
	if schedule == nil {
		ctx, cancel := context.WithCancel(a.daemonCtx)
		schedule = &rateLimitSchedule{cancel: cancel, at: target.ResetAt}
		if schedule.at.Before(time.Now()) {
			schedule.at = time.Now().UTC()
		}
		a.rateLimitSchedules[key] = schedule
		go a.waitForRateLimit(ctx, schedule, projectID, root, chatID, surface, sess.RateLimitNotice, sess.LastUserMessageAt, target)
	}
	a.activeMu.Unlock()
	writeJSON(w, http.StatusAccepted, map[string]string{"scheduledAt": schedule.at.Format(time.RFC3339Nano)})
}

func (a *chatAPI) cancelRateLimitScheduleLocked(key string) {
	if schedule := a.rateLimitSchedules[key]; schedule != nil {
		delete(a.rateLimitSchedules, key)
		schedule.cancel()
	}
}

func (a *chatAPI) waitForRateLimit(ctx context.Context, schedule *rateLimitSchedule, projectID, root, chatID, surface, notice string, lastUserAt time.Time, target chatstore.RateLimitResume) {
	defer schedule.cancel()
	key := projectID + "\x00" + chatID
	for remaining := time.Until(schedule.at); remaining > 0; remaining = time.Until(schedule.at) {
		timer := time.NewTimer(min(remaining, time.Second))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
	a.activeMu.Lock()
	if a.rateLimitSchedules[key] != schedule || ctx.Err() != nil {
		a.activeMu.Unlock()
		return
	}
	delete(a.rateLimitSchedules, key)
	run := newChatRun(a.daemonCtx)
	a.active[key] = run
	a.activeMu.Unlock()
	defer a.finishRun(key, run)
	sess, err := chatstore.ReadSession(projectID, chatID)
	if err != nil || sess.RateLimitNotice != notice || !sess.LastUserMessageAt.Equal(lastUserAt) {
		return
	}
	if err := a.resumeRateLimitedChat(run, projectID, root, surface, sess, target); err != nil {
		run.Emit(cievents.ErrorEvent(1, err.Error()))
	}
}

func (a *chatAPI) resumeRateLimitedChat(run *chatRun, projectID, root, surface string, sess *chatstore.Session, target chatstore.RateLimitResume) error {
	cfg, err := config.LoadOptional()
	if err != nil {
		return err
	}
	prov := config.ProviderByName(cfg, target.Provider)
	if prov == nil {
		return errors.New("the interrupted provider is no longer configured")
	}
	agents, err := rateLimitedSubagents(projectID, sess)
	if err != nil {
		return err
	}
	cfg.Current.Provider, cfg.Current.Model = target.Provider, target.Model
	cfg.ReasoningEffort, cfg.FastMode = target.ReasoningEffort, target.FastMode
	rt := agentruntime.NewRuntime(nil, cfg, prov, projectID, root, sess)
	rt.Out, rt.EventSink, rt.Surface = io.Discard, run, surface
	a.attachBrowser(rt, projectID, sess.ID)
	rt.InitMCP(run.ctx)
	resumeErr := rt.ResumeRateLimitedRun(run.ctx, agents, target.ResumeParent)
	_ = rt.Close()
	latest, err := chatstore.ReadSession(projectID, sess.ID)
	if err != nil {
		return err
	}
	if resumeErr == nil {
		saveRateLimitState(projectID, latest, target, nil)
	} else if notice, reset, limited := llm.RateLimitDetails(resumeErr, time.Now()); limited {
		target.ResetAt = reset
		for _, agent := range agents {
			if current, err := chatstore.FindSubSessionByID(projectID, agent.ID); err == nil && current.RateLimitResetAt.After(target.ResetAt) {
				target.ResetAt = current.RateLimitResetAt
			}
		}
		latest.RateLimitNotice, latest.RateLimitResume = notice, &target
		if err := chatstore.WriteSession(projectID, latest); err != nil {
			return err
		}
	}
	run.Emit(apiEvent("chat_snapshot", map[string]any{"chat": apiChatFromSession(projectID, latest, nil)}))
	return nil
}
