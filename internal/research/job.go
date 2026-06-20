package research

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/apitype"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tokcount"
)

type StartRequest struct {
	Question     string
	Category     string
	ProjectHex   string
	ParentChatID string
	Model        string
	Cfg          *config.Root
	Backend      llm.CompletionBackend
	OnProgress   func(JobRecord, ProgressEvent)
	OnDone       func(JobRecord)
}

type Manager struct {
	mu          sync.Mutex
	runs        map[string]context.CancelFunc
	records     map[string]*JobRecord
	progressKey map[string]string
}

var globalManager = &Manager{
	runs:        map[string]context.CancelFunc{},
	records:     map[string]*JobRecord{},
	progressKey: map[string]string{},
}

func GlobalManager() *Manager {
	return globalManager
}

func (m *Manager) Start(parentCtx context.Context, req StartRequest) (JobRecord, error) {
	question := strings.TrimSpace(req.Question)
	if question == "" {
		return JobRecord{}, fmt.Errorf("empty research query")
	}
	if req.ProjectHex == "" {
		return JobRecord{}, fmt.Errorf("research requires a persisted project session")
	}
	baseSlug := SlugFromQuery(question)
	slug, err := ResolveUniqueSlug(req.ProjectHex, baseSlug)
	if err != nil {
		return JobRecord{}, err
	}
	jobTitle := TitleFromQuery(question)
	id := jobID(req.ParentChatID, question, time.Now().UTC())
	rec := &JobRecord{
		ID:           id,
		Slug:         slug,
		Title:        jobTitle,
		Question:     question,
		Category:     strings.TrimSpace(req.Category),
		Status:       StatusRunning,
		Phase:        PhasePlanning,
		MaxRounds:    config.EffectiveResearchMaxRounds(req.Cfg),
		ParentChatID: req.ParentChatID,
		ProjectHex:   req.ProjectHex,
		StartedAt:    time.Now().UTC(),
	}
	if _, err := chatstore.EnsureResearchDir(req.ProjectHex); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "research ensure dir failed", logging.LogOptions{Params: map[string]any{"project": req.ProjectHex, "err": err.Error()}})
		return JobRecord{}, err
	}
	if err := m.persist(rec); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "research persist job failed", logging.LogOptions{Params: map[string]any{"job_id": id, "err": err.Error()}})
		return JobRecord{}, err
	}
	ctx, cancel := context.WithCancel(parentCtx)
	m.mu.Lock()
	m.runs[id] = cancel
	m.records[id] = rec
	m.mu.Unlock()

	go m.run(ctx, req, rec, nil)
	return *rec, nil
}

func (m *Manager) Resume(parentCtx context.Context, projectHex, target string, req StartRequest) (JobRecord, error) {
	rec, err := m.lookup(projectHex, target)
	if err != nil {
		return JobRecord{}, err
	}
	if rec.Status != StatusPaused {
		return JobRecord{}, fmt.Errorf("research job %s is not paused", rec.ID)
	}
	m.mu.Lock()
	if _, running := m.runs[rec.ID]; running {
		m.mu.Unlock()
		return JobRecord{}, fmt.Errorf("research job %s is already running", rec.ID)
	}
	snapshot := rec
	ptr := &snapshot
	ctx, cancel := context.WithCancel(parentCtx)
	m.runs[rec.ID] = cancel
	m.records[rec.ID] = ptr
	m.mu.Unlock()

	ptr.Status = StatusRunning
	ptr.Error = ""
	_ = m.persist(ptr)
	resume := resumeStateFromRecord(ptr)
	go m.run(ctx, req, ptr, resume)
	return *ptr, nil
}

func resumeStateFromRecord(rec *JobRecord) *EngineResumeState {
	if rec.ResearchPlan == "" && len(rec.Findings) == 0 && rec.Round == 0 && len(rec.QueriesUsed) == 0 {
		return &EngineResumeState{Round: rec.Round}
	}
	return &EngineResumeState{
		Plan:        rec.ResearchPlan,
		Category:    rec.Category,
		Report:      rec.EvolvingReport,
		Findings:    rec.Findings,
		Round:       rec.Round,
		QueriesUsed: rec.QueriesUsed,
		URLsFetched: rec.URLsFetched,
	}
}

func (m *Manager) run(ctx context.Context, req StartRequest, rec *JobRecord, resume *EngineResumeState) {
	defer m.finishRun(rec.ID)
	var usage apitype.UsageStats
	engine := NewEngine(EngineConfig{
		Cfg:      req.Cfg,
		Model:    req.Model,
		Question: rec.Question,
		Category: rec.Category,
		Resume:   resume,
		LLM:      NewBackendLLM(ctx, req.Backend, req.Cfg, req.Model, &usage),
		IsCancelled: func() bool {
			select {
			case <-ctx.Done():
				return true
			default:
				return false
			}
		},
		OnProgress: func(ev ProgressEvent) {
			m.updateProgress(rec, ev, req.OnProgress)
		},
		OnPersist: func(j *JobRecord) { _ = m.persist(j) },
	})
	if resume != nil {
		engine.restoreURLStats(rec.Stats)
		if len(rec.URLAttempts) > 0 {
			engine.urlAttempts = append([]URLAttempt(nil), rec.URLAttempts...)
		}
	}

	markdown, findings, stats, meta, err := engine.Run(ctx)
	rec.Findings = findings
	rec.ResearchPlan = meta.Plan
	rec.EvolvingReport = meta.Report
	stats.Model = req.Model
	if rec.Category == "" {
		rec.Category = meta.Category
	}

	if ctx.Err() != nil {
		rec.Status = StatusCancelled
		rec.Error = ctx.Err().Error()
		rec.Stats = stats
		rec.FinishedAt = time.Now().UTC()
		logging.Log(logging.INFO_LOG_LEVEL, "research job cancelled", logging.LogOptions{Params: map[string]any{"job_id": rec.ID, "slug": rec.Slug}})
		_ = m.persist(rec)
		if req.OnDone != nil {
			req.OnDone(*rec)
		}
		return
	}
	if err != nil {
		cp := engine.CheckpointState()
		applyCheckpoint(rec, cp)
		rec.Findings = findings
		rec.ResearchPlan = meta.Plan
		rec.EvolvingReport = meta.Report
		rec.Stats = stats
		if errors.Is(err, ErrPausedLLM) {
			rec.Status = StatusPaused
			rec.Error = pauseDetail(err)
			logging.Log(logging.WARNING_LOG_LEVEL, "research job paused", logging.LogOptions{Params: map[string]any{"job_id": rec.ID, "slug": rec.Slug, "err": rec.Error}})
			_ = m.persist(rec)
			if req.OnDone != nil {
				req.OnDone(*rec)
			}
			return
		}
		rec.Status = StatusFailed
		rec.Error = err.Error()
		rec.FinishedAt = time.Now().UTC()
		logging.Log(logging.ERROR_LOG_LEVEL, "research job failed", logging.LogOptions{Params: map[string]any{"job_id": rec.ID, "slug": rec.Slug, "err": rec.Error}})
		_ = m.persist(rec)
		if req.OnDone != nil {
			req.OnDone(*rec)
		}
		return
	}

	htmlBody, htmlErr := engine.RenderHTML(rec.Title, markdown, stats)
	if htmlErr != nil {
		rec.Status = StatusFailed
		rec.Error = htmlErr.Error()
		rec.FinishedAt = time.Now().UTC()
		logging.Log(logging.ERROR_LOG_LEVEL, "research HTML render failed", logging.LogOptions{Params: map[string]any{"job_id": rec.ID, "slug": rec.Slug, "err": rec.Error}})
		_ = m.persist(rec)
		if req.OnDone != nil {
			req.OnDone(*rec)
		}
		return
	}
	htmlPath, err := chatstore.ResearchHTMLPath(rec.ProjectHex, rec.Slug)
	if err != nil {
		rec.Status = StatusFailed
		rec.Error = err.Error()
		rec.FinishedAt = time.Now().UTC()
		_ = m.persist(rec)
		if req.OnDone != nil {
			req.OnDone(*rec)
		}
		return
	}
	if err := os.WriteFile(htmlPath, []byte(htmlBody), 0o600); err != nil {
		rec.Status = StatusFailed
		rec.Error = err.Error()
		rec.FinishedAt = time.Now().UTC()
		_ = m.persist(rec)
		if req.OnDone != nil {
			req.OnDone(*rec)
		}
		return
	}

	est := tokcount.TextTokens(markdown, req.Model)
	stats.EstimatedTokens = est
	stats.PromptTokens = usage.PromptTokens
	stats.ResponseTokens = usage.ResponseTokens
	stats.TotalTokens = usage.TotalTokens
	if stats.TotalTokens == 0 {
		stats.TotalTokens = est
	}
	rec.HTMLPath = htmlPath
	rec.Stats = stats
	rec.Status = StatusDone
	rec.Phase = PhaseWriting
	rec.FinishedAt = time.Now().UTC()
	logging.Log(logging.INFO_LOG_LEVEL, "research job complete", logging.LogOptions{Params: map[string]any{"job_id": rec.ID, "slug": rec.Slug, "html_path": rec.HTMLPath}})
	_ = m.persist(rec)
	if req.OnDone != nil {
		req.OnDone(*rec)
	}
}

func (m *Manager) updateProgress(rec *JobRecord, ev ProgressEvent, fn func(JobRecord, ProgressEvent)) {
	if ev.URL != "" || ev.Message == URLAttemptSearchFailed {
		applyURLAttempt(rec, ev)
		_ = m.persist(rec)
		if fn != nil && isURLFailureEvent(ev) {
			fn(*rec, ev)
		}
		return
	}
	key := fmt.Sprintf("%s:%d:%d:%d", ev.Phase, ev.Round, ev.TotalSources, ev.TotalFindings)
	m.mu.Lock()
	if m.progressKey[rec.ID] == key {
		m.mu.Unlock()
		return
	}
	m.progressKey[rec.ID] = key
	m.mu.Unlock()

	rec.Phase = ev.Phase
	if ev.Round > 0 {
		rec.Round = ev.Round
	}
	if ev.MaxRounds > 0 {
		rec.MaxRounds = ev.MaxRounds
	}
	if ev.TotalQueries > 0 {
		rec.Stats.Queries = ev.TotalQueries
	}
	rec.Stats.URLs = ev.TotalSources
	if ev.Phase == PhaseAnalyzing || ev.Phase == PhaseWriting {
		rec.Stats.Findings = ev.TotalFindings
	}
	if ev.Round > 0 {
		rec.Stats.Rounds = ev.Round
	}
	_ = m.persist(rec)
	if fn != nil {
		fn(*rec, ev)
	}
}

func (m *Manager) persist(rec *JobRecord) error {
	return chatstore.WriteResearchJobFile(rec.ProjectHex, rec.Slug, rec)
}

func (m *Manager) finishRun(id string) {
	m.mu.Lock()
	delete(m.runs, id)
	delete(m.progressKey, id)
	m.mu.Unlock()
}

func (m *Manager) Cancel(projectHex, target string) error {
	rec, err := m.lookup(projectHex, target)
	if err != nil {
		return err
	}
	m.mu.Lock()
	cancel := m.runs[rec.ID]
	m.mu.Unlock()
	if cancel == nil {
		return fmt.Errorf("research job %s is not running (use /research delete to remove)", rec.ID)
	}
	cancel()
	return nil
}

func (m *Manager) Delete(projectHex, target string) error {
	rec, err := m.lookup(projectHex, target)
	if err != nil {
		return err
	}
	m.mu.Lock()
	cancel := m.runs[rec.ID]
	m.mu.Unlock()
	if cancel != nil {
		cancel()
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			m.mu.Lock()
			_, running := m.runs[rec.ID]
			m.mu.Unlock()
			if !running {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	m.mu.Lock()
	delete(m.records, rec.ID)
	delete(m.progressKey, rec.ID)
	m.mu.Unlock()
	if err := chatstore.DeleteResearchJob(rec.ProjectHex, rec.Slug); err != nil {
		return err
	}
	return nil
}

func (m *Manager) Get(projectHex, target string) (JobRecord, error) {
	return m.lookup(projectHex, target)
}

func (m *Manager) List(projectHex string) ([]JobRecord, error) {
	slugs, err := chatstore.ListResearchJobFiles(projectHex)
	if err != nil {
		return nil, err
	}
	out := make([]JobRecord, 0, len(slugs))
	for _, slug := range slugs {
		var rec JobRecord
		if err := chatstore.ReadResearchJobFile(projectHex, slug, &rec); err != nil {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (m *Manager) lookup(projectHex, target string) (JobRecord, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return JobRecord{}, fmt.Errorf("research job not found")
	}
	m.mu.Lock()
	for _, rec := range m.records {
		if rec.ProjectHex == projectHex && (rec.ID == target || strings.EqualFold(rec.Title, target) || rec.Slug == target) {
			m.mu.Unlock()
			return *rec, nil
		}
	}
	m.mu.Unlock()
	slugs, err := chatstore.ListResearchJobFiles(projectHex)
	if err != nil {
		return JobRecord{}, err
	}
	for _, slug := range slugs {
		var rec JobRecord
		if err := chatstore.ReadResearchJobFile(projectHex, slug, &rec); err != nil {
			continue
		}
		if rec.ID == target || strings.EqualFold(rec.Title, target) || rec.Slug == target {
			return rec, nil
		}
	}
	return JobRecord{}, fmt.Errorf("research job not found: %s", target)
}

func jobID(parentChatID, question string, t time.Time) string {
	s := parentChatID + "\x00" + question + "\x00" + t.UTC().Format(time.RFC3339Nano)
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func FormatProgressLine(rec JobRecord, ev ProgressEvent) string {
	if ev.URL != "" || ev.Message == URLAttemptSearchFailed {
		return FormatURLAttemptLine(rec, ev)
	}
	title := strings.TrimSpace(rec.Title)
	if title == "" {
		title = rec.Slug
	}
	switch ev.Phase {
	case PhasePlanning:
		return fmt.Sprintf("research %s — planning", title)
	case PhaseSearching:
		if ev.Round > 0 {
			line := fmt.Sprintf("research %s — round %d/%d — searching", title, ev.Round, rec.MaxRounds)
			if ev.QueryPreview != "" {
				line += " — " + ev.QueryPreview
			}
			return line
		}
		return fmt.Sprintf("research %s — searching", title)
	case PhaseReading:
		return fmt.Sprintf("research %s — round %d/%d — reading (%d sources)", title, ev.Round, rec.MaxRounds, ev.TotalSources)
	case PhaseAnalyzing:
		return fmt.Sprintf("research %s — round %d/%d — analyzing (%d findings)", title, ev.Round, rec.MaxRounds, ev.TotalFindings)
	case PhaseWriting:
		return fmt.Sprintf("research %s — writing report", title)
	default:
		return fmt.Sprintf("research %s — %s", title, ev.Phase)
	}
}

func FormatJobStatsLine(rec JobRecord) string {
	q, u, f := rec.Stats.Queries, rec.Stats.URLs, rec.Stats.Findings
	if f == 0 && len(rec.Findings) > 0 {
		f = len(rec.Findings)
	}
	if rec.Status != StatusRunning && rec.Status != StatusPaused && q == 0 && u == 0 && f == 0 && FormatURLFailureSummary(rec.Stats) == "" {
		return ""
	}
	line := fmt.Sprintf("%d queries · %d urls · %d findings", q, u, f)
	if summary := FormatURLFailureSummary(rec.Stats); summary != "" {
		line += " · " + summary
	}
	return line
}

func FormatPausedMessage(rec JobRecord) string {
	title := strings.TrimSpace(rec.Title)
	if title == "" {
		title = rec.Slug
	}
	msg := fmt.Sprintf("research %s paused (LLM unavailable)", title)
	if rec.Error != "" {
		msg += "\n\t" + FormatResearchError(rec.Error)
	}
	msg += "\n\tresume with /research resume " + rec.ID
	return msg
}

func pauseDetail(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	prefix := ErrPausedLLM.Error()
	if strings.HasPrefix(msg, prefix) {
		msg = strings.TrimSpace(strings.TrimPrefix(msg, prefix))
		msg = strings.TrimPrefix(msg, ":")
		return strings.TrimSpace(msg)
	}
	return msg
}

func FormatDoneMessage(rec JobRecord) string {
	title := strings.TrimSpace(rec.Title)
	if title == "" {
		title = rec.Slug
	}
	msg := fmt.Sprintf("research %s done\n\t%s", title, rec.HTMLPath)
	if rec.Stats.DurationSecs > 0 {
		msg += fmt.Sprintf("\n\t%.1fs · %d rounds · %d urls", rec.Stats.DurationSecs, rec.Stats.Rounds, rec.Stats.URLs)
	}
	if rec.Stats.TotalTokens > 0 || rec.Stats.EstimatedTokens > 0 {
		tok := rec.Stats.TotalTokens
		if tok == 0 {
			tok = rec.Stats.EstimatedTokens
		}
		msg += fmt.Sprintf("\n\t~%d tokens (estimated)", tok)
	}
	return msg
}
