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
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/search"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tokcount"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/webfetch"
)

type StartRequest struct {
	Question     string
	Category     string
	ProjectHex   string
	ParentChatID string
	Model        string
	Cfg          *config.Root
	Backend      llm.CompletionBackend
	Search       search.Engine
	Fetch        webfetch.Fetcher
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
	if err := m.persist(*rec); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "research persist job failed", logging.LogOptions{Params: map[string]any{"job_id": id, "err": err.Error()}})
		return JobRecord{}, err
	}
	ctx, cancel := context.WithCancel(parentCtx)
	stored := cloneJobRecord(*rec)
	m.mu.Lock()
	m.runs[id] = cancel
	m.records[id] = &stored
	m.mu.Unlock()

	go m.run(ctx, req, id, nil)
	return cloneJobRecord(stored), nil
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
	if current, ok := m.records[rec.ID]; ok {
		rec = cloneJobRecord(*current)
	}
	snapshot := cloneJobRecord(rec)
	snapshot.Status = StatusRunning
	snapshot.Error = ""
	ctx, cancel := context.WithCancel(parentCtx)
	m.runs[rec.ID] = cancel
	m.records[rec.ID] = &snapshot
	returned := cloneJobRecord(snapshot)
	m.mu.Unlock()

	_ = m.persist(snapshot)
	resume := resumeStateFromRecord(&snapshot)
	go m.run(ctx, req, snapshot.ID, resume)
	return returned, nil
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

func (m *Manager) run(ctx context.Context, req StartRequest, jobID string, resume *EngineResumeState) {
	defer m.finishRun(jobID)
	rec, ok := m.snapshotRecord(jobID)
	if !ok {
		return
	}
	var usage apitype.UsageStats
	engine := NewEngine(EngineConfig{
		Cfg:      req.Cfg,
		Model:    req.Model,
		Question: rec.Question,
		Category: rec.Category,
		Resume:   resume,
		LLM:      NewBackendLLM(ctx, req.Backend, req.Cfg, req.Model, &usage),
		Search:   req.Search,
		Fetch:    req.Fetch,
		IsCancelled: func() bool {
			select {
			case <-ctx.Done():
				return true
			default:
				return false
			}
		},
		OnProgress: func(ev ProgressEvent) {
			m.updateProgress(jobID, ev, req.OnProgress)
		},
		OnPersist: func(_ *JobRecord) {
			if snapshot, ok := m.snapshotRecord(jobID); ok {
				_ = m.persist(snapshot)
			}
		},
	})
	if resume != nil {
		engine.restoreURLStats(rec.Stats)
		if len(rec.URLAttempts) > 0 {
			engine.urlAttempts = append([]URLAttempt(nil), rec.URLAttempts...)
		}
	}

	markdown, findings, stats, meta, err := engine.Run(ctx)
	stats.Model = req.Model

	if ctx.Err() != nil {
		snapshot, ok := m.mutateRecord(jobID, func(current *JobRecord) {
			applyEngineResults(current, findings, meta, stats)
			current.Status = StatusCancelled
			current.Error = ctx.Err().Error()
			current.FinishedAt = time.Now().UTC()
		})
		if !ok {
			return
		}
		logging.Log(logging.INFO_LOG_LEVEL, "research job cancelled", logging.LogOptions{Params: map[string]any{"job_id": snapshot.ID, "slug": snapshot.Slug}})
		_ = m.persist(snapshot)
		if req.OnDone != nil {
			req.OnDone(snapshot)
		}
		return
	}
	if err != nil {
		cp := engine.CheckpointState()
		if errors.Is(err, ErrPausedLLM) {
			detail := pauseDetail(err)
			snapshot, ok := m.mutateRecord(jobID, func(current *JobRecord) {
				applyCheckpoint(current, cp)
				applyEngineResults(current, findings, meta, stats)
				current.Status = StatusPaused
				current.Error = detail
			})
			if !ok {
				return
			}
			logging.Log(logging.WARNING_LOG_LEVEL, "research job paused", logging.LogOptions{Params: map[string]any{"job_id": snapshot.ID, "slug": snapshot.Slug, "err": snapshot.Error}})
			_ = m.persist(snapshot)
			if req.OnDone != nil {
				req.OnDone(snapshot)
			}
			return
		}
		detail := err.Error()
		snapshot, ok := m.mutateRecord(jobID, func(current *JobRecord) {
			applyCheckpoint(current, cp)
			applyEngineResults(current, findings, meta, stats)
			current.Status = StatusFailed
			current.Error = detail
			current.FinishedAt = time.Now().UTC()
		})
		if !ok {
			return
		}
		logging.Log(logging.ERROR_LOG_LEVEL, "research job failed", logging.LogOptions{Params: map[string]any{"job_id": snapshot.ID, "slug": snapshot.Slug, "err": snapshot.Error}})
		_ = m.persist(snapshot)
		if req.OnDone != nil {
			req.OnDone(snapshot)
		}
		return
	}

	htmlBody, htmlErr := engine.RenderHTML(rec.Title, markdown, stats)
	if htmlErr != nil {
		detail := htmlErr.Error()
		snapshot, ok := m.mutateRecord(jobID, func(current *JobRecord) {
			applyEngineResults(current, findings, meta, stats)
			current.Status = StatusFailed
			current.Error = detail
			current.FinishedAt = time.Now().UTC()
		})
		if !ok {
			return
		}
		logging.Log(logging.ERROR_LOG_LEVEL, "research HTML render failed", logging.LogOptions{Params: map[string]any{"job_id": snapshot.ID, "slug": snapshot.Slug, "err": snapshot.Error}})
		_ = m.persist(snapshot)
		if req.OnDone != nil {
			req.OnDone(snapshot)
		}
		return
	}
	htmlPath, err := chatstore.ResearchHTMLPath(rec.ProjectHex, rec.Slug)
	if err != nil {
		detail := err.Error()
		snapshot, ok := m.mutateRecord(jobID, func(current *JobRecord) {
			applyEngineResults(current, findings, meta, stats)
			current.Status = StatusFailed
			current.Error = detail
			current.FinishedAt = time.Now().UTC()
		})
		if !ok {
			return
		}
		_ = m.persist(snapshot)
		if req.OnDone != nil {
			req.OnDone(snapshot)
		}
		return
	}
	if err := os.WriteFile(htmlPath, []byte(htmlBody), 0o600); err != nil {
		detail := err.Error()
		snapshot, ok := m.mutateRecord(jobID, func(current *JobRecord) {
			applyEngineResults(current, findings, meta, stats)
			current.Status = StatusFailed
			current.Error = detail
			current.FinishedAt = time.Now().UTC()
		})
		if !ok {
			return
		}
		_ = m.persist(snapshot)
		if req.OnDone != nil {
			req.OnDone(snapshot)
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
	snapshot, ok := m.mutateRecord(jobID, func(current *JobRecord) {
		applyEngineResults(current, findings, meta, stats)
		current.HTMLPath = htmlPath
		current.Status = StatusDone
		current.Phase = PhaseWriting
		current.FinishedAt = time.Now().UTC()
	})
	if !ok {
		return
	}
	logging.Log(logging.INFO_LOG_LEVEL, "research job complete", logging.LogOptions{Params: map[string]any{"job_id": snapshot.ID, "slug": snapshot.Slug, "html_path": snapshot.HTMLPath}})
	_ = m.persist(snapshot)
	if req.OnDone != nil {
		req.OnDone(snapshot)
	}
}

func applyEngineResults(rec *JobRecord, findings []Finding, meta RunMeta, stats JobStats) {
	rec.Findings = append([]Finding(nil), findings...)
	rec.ResearchPlan = meta.Plan
	rec.EvolvingReport = meta.Report
	rec.Stats = stats
	if rec.Category == "" {
		rec.Category = meta.Category
	}
}

func (m *Manager) snapshotRecord(id string) (JobRecord, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[id]
	if !ok || rec == nil {
		return JobRecord{}, false
	}
	return cloneJobRecord(*rec), true
}

func (m *Manager) mutateRecord(id string, fn func(*JobRecord)) (JobRecord, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[id]
	if !ok || rec == nil {
		return JobRecord{}, false
	}
	next := cloneJobRecord(*rec)
	fn(&next)
	m.records[id] = &next
	return cloneJobRecord(next), true
}

func (m *Manager) updateProgress(jobID string, ev ProgressEvent, fn func(JobRecord, ProgressEvent)) {
	var snapshot JobRecord
	m.mu.Lock()
	rec, ok := m.records[jobID]
	if !ok || rec == nil {
		m.mu.Unlock()
		return
	}
	next := cloneJobRecord(*rec)
	if ev.URL != "" || ev.Message == URLAttemptSearchFailed {
		applyURLAttempt(&next, ev)
	} else {
		key := fmt.Sprintf("%s:%d:%d:%d", ev.Phase, ev.Round, ev.TotalSources, ev.TotalFindings)
		if m.progressKey[jobID] == key {
			m.mu.Unlock()
			return
		}
		m.progressKey[jobID] = key

		next.Phase = ev.Phase
		if ev.Round > 0 {
			next.Round = ev.Round
		}
		if ev.MaxRounds > 0 {
			next.MaxRounds = ev.MaxRounds
		}
		if ev.TotalQueries > 0 {
			next.Stats.Queries = ev.TotalQueries
		}
		next.Stats.URLs = ev.TotalSources
		if ev.Phase == PhaseAnalyzing || ev.Phase == PhaseWriting {
			next.Stats.Findings = ev.TotalFindings
		}
		if ev.Round > 0 {
			next.Stats.Rounds = ev.Round
		}
	}
	m.records[jobID] = &next
	snapshot = cloneJobRecord(next)
	m.mu.Unlock()

	_ = m.persist(snapshot)
	if fn != nil && (ev.URL == "" || isURLFailureEvent(ev)) {
		fn(snapshot, ev)
	}
}

func (m *Manager) persist(rec JobRecord) error {
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
			snapshot := cloneJobRecord(*rec)
			m.mu.Unlock()
			return snapshot, nil
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
