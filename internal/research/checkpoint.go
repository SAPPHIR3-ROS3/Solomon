package research

import "strings"

func fallbackQueries(question string, round int) []string {
	q := strings.TrimSpace(question)
	if q == "" {
		return nil
	}
	if round == 1 {
		return []string{q, q + " 2025", q + " guide recommendations", q + " comparison review"}
	}
	return []string{q + " additional sources", q + " latest update"}
}

func (e *Engine) applyResume(r *EngineResumeState) {
	e.plan = r.Plan
	if r.Category != "" {
		e.category = r.Category
	}
	e.report = r.Report
	if len(r.Findings) > 0 {
		e.findings = append([]Finding(nil), r.Findings...)
	}
	if r.Round > 0 {
		e.roundCount = r.Round
	}
	for _, q := range r.QueriesUsed {
		e.queriesUsed[q] = struct{}{}
	}
	for _, u := range r.URLsFetched {
		e.urlsFetched[u] = struct{}{}
	}
}

func (e *Engine) CheckpointState() EngineResumeState {
	queries := make([]string, 0, len(e.queriesUsed))
	for q := range e.queriesUsed {
		queries = append(queries, q)
	}
	urls := make([]string, 0, len(e.urlsFetched))
	for u := range e.urlsFetched {
		urls = append(urls, u)
	}
	return EngineResumeState{
		Plan:        e.plan,
		Category:    e.category,
		Report:      e.report,
		Findings:    append([]Finding(nil), e.findings...),
		Round:       e.roundCount,
		QueriesUsed: queries,
		URLsFetched: urls,
	}
}

func applyCheckpoint(rec *JobRecord, cp EngineResumeState) {
	rec.ResearchPlan = cp.Plan
	if cp.Category != "" {
		rec.Category = cp.Category
	}
	rec.EvolvingReport = cp.Report
	rec.Findings = cp.Findings
	rec.Round = cp.Round
	rec.QueriesUsed = cp.QueriesUsed
	rec.URLsFetched = cp.URLsFetched
}
