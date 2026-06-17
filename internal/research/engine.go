package research

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	researchhtml "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/research/html"
)

type pageCandidate struct {
	URL   string
	Title string
}

type EngineConfig struct {
	Cfg         *config.Root
	Model       string
	Question    string
	Category    string
	LLM         LLMCaller
	Resume      *EngineResumeState
	OnProgress  ProgressFn
	IsCancelled CancelFn
	OnPersist   func(*JobRecord)
}

type Engine struct {
	cfg            EngineConfig
	maxRounds      int
	minRounds      int
	maxURLsPerRound int
	maxContentChars int
	maxTime        time.Duration
	maxEmptyRounds int
	synthesisWindow int

	queriesUsed map[string]struct{}
	urlsFetched map[string]struct{}
	findings    []Finding
	plan        string
	report      string
	category    string
	roundCount  int
	startTime   time.Time
	searchEngine  string
	lastSearchErr string
	lastLLMErr    string
	urlAttempts   []URLAttempt
	urlReadOK     int
	urlFetchFailed int
	urlEmptyContent int
	urlLLMFailed  int
	urlLowQuality int
	urlParseFailed int
	searchFailures int
}

func (e *Engine) restoreURLStats(st JobStats) {
	e.urlReadOK = st.URLReadOK
	e.urlFetchFailed = st.URLFetchFailed
	e.urlEmptyContent = st.URLEmptyContent
	e.urlLLMFailed = st.URLLLMFailed
	e.urlLowQuality = st.URLLowQuality
	e.urlParseFailed = st.URLParseFailed
	e.searchFailures = st.SearchFailures
}

func NewEngine(cfg EngineConfig) *Engine {
	root := cfg.Cfg
	maxSec := config.ResearchMaxTimeSeconds(root)
	return &Engine{
		cfg:             cfg,
		maxRounds:       config.EffectiveResearchMaxRounds(root),
		minRounds:       config.DefaultResearchMinRounds,
		maxURLsPerRound: config.EffectiveResearchMaxURLsPerRound(root),
		maxContentChars: config.EffectiveResearchMaxContentChars(root),
		maxTime:         time.Duration(maxSec) * time.Second,
		maxEmptyRounds:  2,
		synthesisWindow: 10,
		queriesUsed:     map[string]struct{}{},
		urlsFetched:     map[string]struct{}{},
		category:        strings.TrimSpace(cfg.Category),
	}
}

type RunMeta struct {
	Plan     string
	Category string
	Report   string
}

func (e *Engine) Run(ctx context.Context) (markdown string, findings []Finding, stats JobStats, meta RunMeta, err error) {
	e.startTime = time.Now()
	question := strings.TrimSpace(e.cfg.Question)
	if question == "" {
		return "", nil, JobStats{}, RunMeta{}, fmt.Errorf("empty research question")
	}

	startRound := 1
	if r := e.cfg.Resume; r != nil {
		e.applyResume(r)
		if r.Round > 0 {
			startRound = r.Round
		}
	} else {
		e.emit(ProgressEvent{Phase: PhasePlanning})
		planRaw, _, planErr := e.cfg.LLM.Complete(fmt.Sprintf(researchPlanPrompt, question), 1024)
		e.noteLLMErr(planErr)
		if planErr == nil {
			e.plan = planFromJSON(parseJSONObject(planRaw))
			if e.plan == "" {
				e.plan = planRaw
			}
		}
	}

	if e.category == "" {
		catRaw, _, catErr := e.cfg.LLM.Complete(fmt.Sprintf(classifyCategoryPrompt, question), 32)
		e.noteLLMErr(catErr)
		e.category = normalizeCategory(catRaw)
	}

	consecutiveEmpty := 0
	for round := startRound; round <= e.maxRounds; round++ {
		e.roundCount = round
		if e.cancelled() {
			meta := RunMeta{Plan: e.plan, Category: e.category, Report: e.report}
			return e.report, e.findings, e.buildStats(), meta, context.Canceled
		}
		if e.timeExceeded() {
			break
		}

		queries := e.generateQueries(ctx, question, round)
		if len(queries) == 0 {
			if e.lastLLMErr != "" {
				meta := RunMeta{Plan: e.plan, Category: e.category, Report: e.report}
				return "", nil, e.buildStats(), meta, pauseLLMError(e.lastLLMErr)
			}
			break
		}
		preview := ""
		if len(queries) > 0 {
			preview = queries[0]
		}
		e.emit(ProgressEvent{Phase: PhaseSearching, Round: round, MaxRounds: e.maxRounds, QueryPreview: preview})

		roundFindings, newURLs := e.searchAndExtract(ctx, question, queries, round)
		if len(roundFindings) > 0 {
			e.findings = append(e.findings, roundFindings...)
			consecutiveEmpty = 0
		} else if newURLs == 0 {
			consecutiveEmpty++
			if consecutiveEmpty >= e.maxEmptyRounds {
				if len(e.findings) == 0 {
					meta := RunMeta{Plan: e.plan, Category: e.category, Report: e.report}
					errMsg := "web search returned no results"
					if e.lastSearchErr != "" {
						errMsg += ": " + e.lastSearchErr
					}
					return "", nil, e.buildStats(), meta, fmt.Errorf("%s", errMsg)
				}
				break
			}
		}

		if len(e.findings) > 0 {
			e.emit(ProgressEvent{Phase: PhaseAnalyzing, Round: round, MaxRounds: e.maxRounds})
			report, synErr := e.synthesize(question)
			if synErr == nil && strings.TrimSpace(report) != "" {
				e.report = report
			}
		}

		if round >= e.minRounds && strings.TrimSpace(e.report) != "" {
			stopRaw, _, _ := e.cfg.LLM.Complete(fmt.Sprintf(stopPrompt, question, e.report, round), 128)
			if shouldStopFromLLM(stopRaw) {
				break
			}
		}
	}

	if strings.TrimSpace(e.report) == "" {
		if len(e.findings) > 0 {
			e.report = e.fallbackReport(question)
		} else {
			meta := RunMeta{Plan: e.plan, Category: e.category, Report: e.report}
			if e.lastLLMErr != "" {
				return "", nil, e.buildStats(), meta, pauseLLMError(e.lastLLMErr)
			}
			return "", nil, e.buildStats(), meta, fmt.Errorf("%s", e.gatherFailureMessage())
		}
	}

	e.emit(ProgressEvent{Phase: PhaseWriting})
	final, err := e.finalReport(question, e.report)
	if err != nil {
		final = e.report
	}
	tldr, _, _ := e.cfg.LLM.Complete(fmt.Sprintf(tldrPrompt, question, final), 1024)
	markdown = AppendTLDRSection(final, tldr)
	meta = RunMeta{Plan: e.plan, Category: e.category, Report: e.report}
	return markdown, e.findings, e.buildStats(), meta, nil
}

func (e *Engine) RenderHTML(title, markdown string, stats JobStats) (string, error) {
	return researchhtml.Render(researchhtml.Input{
		Title:    title,
		Question: e.cfg.Question,
		Markdown: markdown,
		Findings: toHTMLFindings(e.findings),
		Stats:    toHTMLStats(stats),
	})
}

func toHTMLFindings(in []Finding) []researchhtml.Source {
	out := make([]researchhtml.Source, 0, len(in))
	seen := map[string]struct{}{}
	for _, f := range in {
		if f.URL == "" {
			continue
		}
		if _, ok := seen[f.URL]; ok {
			continue
		}
		seen[f.URL] = struct{}{}
		out = append(out, researchhtml.Source{URL: f.URL, Title: f.Title})
	}
	return out
}

func toHTMLStats(s JobStats) researchhtml.Stats {
	return researchhtml.Stats{
		DurationSecs: s.DurationSecs,
		Rounds:       s.Rounds,
		Queries:      s.Queries,
		URLs:         s.URLs,
		Findings:     s.Findings,
		SearchEngine: s.SearchEngine,
		Model:        s.Model,
	}
}

func (e *Engine) generateQueries(_ context.Context, question string, round int) []string {
	num := 3
	instruction := "We already have partial findings. Generate targeted follow-up queries to fill gaps, verify claims, or explore specific aspects that the report doesn't yet cover well."
	if round == 1 {
		num = 4
		instruction = "This is the first round — generate broad, diverse queries that explore the key facets of the question."
	}
	plan := e.plan
	if plan == "" {
		plan = "(No plan — search broadly.)"
	}
	report := e.report
	if report == "" {
		report = "(No findings yet.)"
	}
	prompt := fmt.Sprintf(queryGenPrompt, question, plan, report, round, num, instruction)
	raw, _, err := e.cfg.LLM.Complete(prompt, 4096)
	e.noteLLMErr(err)
	queries := parseJSONArray(raw)
	var out []string
	for _, q := range queries {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		if _, ok := e.queriesUsed[q]; ok {
			continue
		}
		e.queriesUsed[q] = struct{}{}
		out = append(out, q)
	}
	if len(out) == 0 {
		for _, q := range fallbackQueries(question, round) {
			if _, ok := e.queriesUsed[q]; ok {
				continue
			}
			e.queriesUsed[q] = struct{}{}
			out = append(out, q)
		}
	}
	return out
}

func (e *Engine) searchAndExtract(ctx context.Context, question string, queries []string, round int) ([]Finding, int) {
	engine := e.cfg.Cfg.EffectiveWebSearchEngine()
	e.searchEngine = engine
	limit := e.maxURLsPerRound * len(queries)
	if limit < e.maxURLsPerRound {
		limit = e.maxURLsPerRound
	}

	var urls []pageCandidate

	for _, query := range queries {
		if e.cancelled() || e.timeExceeded() {
			break
		}
		resp, err := runSearch(ctx, e.cfg.Cfg, engine, query, 10)
		if err != nil {
			e.lastSearchErr = err.Error()
			e.recordSearchFailure(query, err)
			continue
		}
		if len(resp.Hits) == 0 {
			continue
		}
		for _, hit := range resp.Hits {
			if len(urls) >= limit {
				break
			}
			url := strings.TrimSpace(hit.URL)
			if url == "" {
				continue
			}
			if _, ok := e.urlsFetched[url]; ok {
				continue
			}
			e.urlsFetched[url] = struct{}{}
			urls = append(urls, pageCandidate{URL: url, Title: hit.Title})
		}
		if len(urls) >= limit {
			break
		}
	}

	newURLs := len(urls)
	if newURLs > 0 {
		e.emit(ProgressEvent{Phase: PhaseReading, Round: round, MaxRounds: e.maxRounds})
	}

	if newURLs == 0 || e.cancelled() || e.timeExceeded() {
		return nil, newURLs
	}

	var out []Finding
	for _, u := range urls {
		if e.cancelled() || e.timeExceeded() {
			break
		}
		f, ok := e.fetchAndExtract(ctx, question, u.URL, u.Title)
		if ok {
			out = append(out, f)
		}
	}
	return out, newURLs
}

func (e *Engine) fetchAndExtract(ctx context.Context, question, pageURL, title string) (Finding, bool) {
	page, err := fetchPage(ctx, e.cfg.Cfg, pageURL)
	if err != nil {
		e.recordURLAttempt(pageURL, URLAttemptFetchFailed, err.Error())
		return Finding{}, false
	}
	if strings.TrimSpace(page.Markdown) == "" {
		e.recordURLAttempt(pageURL, URLEmptyContent, "empty markdown after fetch")
		return Finding{}, false
	}
	content := truncateContent(page.Markdown, e.maxContentChars)
	prompt := fmt.Sprintf(extractorPrompt, question, content)
	raw, _, err := e.cfg.LLM.Complete(prompt, 2048)
	e.noteLLMErr(err)
	if err != nil {
		e.recordURLAttempt(pageURL, URLAttemptLLMFailed, err.Error())
		return Finding{}, false
	}
	obj := parseJSONObject(raw)
	if obj != nil {
		summary, _ := obj["summary"].(string)
		if isLowQuality(summary) {
			detail := strings.TrimSpace(summary)
			if detail == "" {
				detail = "insufficient or irrelevant content"
			}
			e.recordURLAttempt(pageURL, URLAttemptLowQuality, detail)
			return Finding{}, false
		}
		evidence, _ := obj["evidence"].(string)
		rational, _ := obj["rational"].(string)
		t := title
		if t == "" {
			t = page.Title
		}
		e.recordURLAttempt(pageURL, URLAttemptOK, "")
		return Finding{URL: pageURL, Title: t, Summary: summary, Evidence: evidence, Rational: rational}, true
	}
	if strings.TrimSpace(raw) == "" {
		e.recordURLAttempt(pageURL, URLAttemptParseFailed, "empty LLM response")
		return Finding{}, false
	}
	t := title
	if t == "" {
		t = page.Title
	}
	ev := raw
	if len(ev) > 3000 {
		ev = ev[:3000]
	}
	sum := raw
	if len(sum) > 500 {
		sum = sum[:500]
	}
	e.recordURLAttempt(pageURL, URLAttemptOK, "raw extraction fallback")
	return Finding{URL: pageURL, Title: t, Evidence: ev, Summary: sum, Rational: "LLM extraction (raw)"}, true
}

func (e *Engine) synthesize(question string) (string, error) {
	window := e.findings
	if len(window) > e.synthesisWindow {
		window = window[len(window)-e.synthesisWindow:]
	}
	report := e.report
	if report == "" {
		report = "(First round — no report yet.)"
	}
	prompt := fmt.Sprintf(synthesizePrompt, question, report, formatFindings(window))
	text, _, err := e.cfg.LLM.Complete(prompt, 8192)
	return text, err
}

func (e *Engine) finalReport(question, report string) (string, error) {
	prompt := fmt.Sprintf(finalReportPrompt, question, report)
	if extra, ok := categoryOverrides[e.category]; ok {
		prompt += "\n\n" + extra
	}
	text, _, err := e.cfg.LLM.Complete(prompt, 8192)
	return text, err
}

func (e *Engine) fallbackReport(question string) string {
	return "# Research findings\n\n**Question:** " + question + "\n\n" + formatFindings(e.findings)
}

func (e *Engine) buildStats() JobStats {
	elapsed := time.Since(e.startTime).Seconds()
	return JobStats{
		DurationSecs:    elapsed,
		Rounds:          e.roundCount,
		Queries:         len(e.queriesUsed),
		URLs:            len(e.urlsFetched),
		Findings:        len(e.findings),
		URLReadOK:       e.urlReadOK,
		URLFetchFailed:  e.urlFetchFailed,
		URLEmptyContent: e.urlEmptyContent,
		URLLLMFailed:    e.urlLLMFailed,
		URLLowQuality:   e.urlLowQuality,
		URLParseFailed:  e.urlParseFailed,
		SearchFailures:  e.searchFailures,
		SearchEngine:    e.searchEngine,
		Model:           e.cfg.Model,
		Category:        e.category,
	}
}

func (e *Engine) emit(ev ProgressEvent) {
	ev.TotalQueries = len(e.queriesUsed)
	ev.TotalSources = len(e.urlsFetched)
	if ev.Phase == PhaseAnalyzing || ev.Phase == PhaseWriting {
		ev.TotalFindings = len(e.findings)
	}
	if e.cfg.OnProgress != nil {
		e.cfg.OnProgress(ev)
	}
}

func (e *Engine) cancelled() bool {
	return e.cfg.IsCancelled != nil && e.cfg.IsCancelled()
}

func (e *Engine) timeExceeded() bool {
	return time.Since(e.startTime) > e.maxTime
}

func (e *Engine) noteLLMErr(err error) {
	if err != nil {
		e.lastLLMErr = err.Error()
	}
}

func (e *Engine) gatherFailureMessage() string {
	if e.lastLLMErr != "" {
		return "no information gathered: LLM failed — " + e.lastLLMErr
	}
	if e.lastSearchErr != "" {
		return "no information gathered: web search failed — " + e.lastSearchErr
	}
	return "no information gathered"
}
