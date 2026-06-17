package research

import (
	"fmt"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

const (
	URLAttemptOK           = "ok"
	URLAttemptFetchFailed  = "fetch_failed"
	URLEmptyContent        = "empty_content"
	URLAttemptLLMFailed    = "llm_failed"
	URLAttemptLowQuality   = "low_quality"
	URLAttemptParseFailed  = "parse_failed"
	URLAttemptSearchFailed = "search_failed"
)

type URLAttempt struct {
	URL    string    `json:"url"`
	Status string    `json:"status"`
	Detail string    `json:"detail,omitempty"`
	At     time.Time `json:"at,omitempty"`
}

func (e *Engine) recordURLAttempt(pageURL, status, detail string) {
	detail = compactResearchError(detail)
	if len(e.urlAttempts) < 300 {
		e.urlAttempts = append(e.urlAttempts, URLAttempt{
			URL:    pageURL,
			Status: status,
			Detail: detail,
			At:     time.Now().UTC(),
		})
	}
	switch status {
	case URLAttemptOK:
		e.urlReadOK++
	case URLAttemptFetchFailed:
		e.urlFetchFailed++
	case URLEmptyContent:
		e.urlEmptyContent++
	case URLAttemptLLMFailed:
		e.urlLLMFailed++
	case URLAttemptLowQuality:
		e.urlLowQuality++
	case URLAttemptParseFailed:
		e.urlParseFailed++
	}
	logging.Log(logging.WARNING_LOG_LEVEL, "research url "+status, logging.LogOptions{Params: map[string]any{
		"url":    pageURL,
		"detail": detail,
		"round":  e.roundCount,
	}})
	if e.cfg.OnProgress != nil {
		e.cfg.OnProgress(ProgressEvent{
			Phase:     PhaseReading,
			Round:     e.roundCount,
			MaxRounds: e.maxRounds,
			URL:       pageURL,
			Message:   status,
			Title:     detail,
		})
	}
}

func (e *Engine) recordSearchFailure(query string, err error) {
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	e.searchFailures++
	logging.Log(logging.WARNING_LOG_LEVEL, "research search failed", logging.LogOptions{Params: map[string]any{
		"query": query,
		"err":   detail,
		"round": e.roundCount,
	}})
	if e.cfg.OnProgress != nil {
		e.cfg.OnProgress(ProgressEvent{
			Phase:        PhaseSearching,
			Round:        e.roundCount,
			MaxRounds:    e.maxRounds,
			QueryPreview: query,
			Message:      URLAttemptSearchFailed,
			Title:        compactResearchError(detail),
		})
	}
}

func applyURLAttempt(rec *JobRecord, ev ProgressEvent) {
	if ev.URL == "" && ev.Message != URLAttemptSearchFailed {
		return
	}
	if ev.URL != "" {
		if len(rec.URLAttempts) < 300 {
			rec.URLAttempts = append(rec.URLAttempts, URLAttempt{
				URL:    ev.URL,
				Status: ev.Message,
				Detail: ev.Title,
				At:     time.Now().UTC(),
			})
		}
	}
	switch ev.Message {
	case URLAttemptOK:
		rec.Stats.URLReadOK++
	case URLAttemptFetchFailed:
		rec.Stats.URLFetchFailed++
	case URLEmptyContent:
		rec.Stats.URLEmptyContent++
	case URLAttemptLLMFailed:
		rec.Stats.URLLLMFailed++
	case URLAttemptLowQuality:
		rec.Stats.URLLowQuality++
	case URLAttemptParseFailed:
		rec.Stats.URLParseFailed++
	case URLAttemptSearchFailed:
		rec.Stats.SearchFailures++
	}
}

func FormatURLAttemptLine(rec JobRecord, ev ProgressEvent) string {
	title := strings.TrimSpace(rec.Title)
	if title == "" {
		title = rec.Slug
	}
	if ev.Message == URLAttemptSearchFailed {
		q := strings.TrimSpace(ev.QueryPreview)
		if q == "" {
			q = "query"
		}
		line := fmt.Sprintf("research %s — search failed — %s", title, q)
		if d := strings.TrimSpace(ev.Title); d != "" {
			line += " — " + d
		}
		return line
	}
	line := fmt.Sprintf("research %s — %s — %s", title, urlAttemptLabel(ev.Message), ev.URL)
	if d := strings.TrimSpace(ev.Title); d != "" {
		line += " — " + d
	}
	return line
}

func urlAttemptLabel(status string) string {
	switch status {
	case URLAttemptFetchFailed:
		return "fetch failed"
	case URLEmptyContent:
		return "empty page"
	case URLAttemptLLMFailed:
		return "LLM extract failed"
	case URLAttemptLowQuality:
		return "low quality"
	case URLAttemptParseFailed:
		return "parse failed"
	case URLAttemptOK:
		return "read ok"
	default:
		return status
	}
}

func FormatURLFailureSummary(st JobStats) string {
	var parts []string
	if st.URLFetchFailed > 0 {
		parts = append(parts, fmt.Sprintf("%d fetch", st.URLFetchFailed))
	}
	if st.URLEmptyContent > 0 {
		parts = append(parts, fmt.Sprintf("%d empty", st.URLEmptyContent))
	}
	if st.URLLLMFailed > 0 {
		parts = append(parts, fmt.Sprintf("%d llm", st.URLLLMFailed))
	}
	if st.URLLowQuality > 0 {
		parts = append(parts, fmt.Sprintf("%d low quality", st.URLLowQuality))
	}
	if st.URLParseFailed > 0 {
		parts = append(parts, fmt.Sprintf("%d parse", st.URLParseFailed))
	}
	if st.SearchFailures > 0 {
		parts = append(parts, fmt.Sprintf("%d search", st.SearchFailures))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ") + " failed"
}

func isURLFailureEvent(ev ProgressEvent) bool {
	switch ev.Message {
	case URLAttemptFetchFailed, URLEmptyContent, URLAttemptLLMFailed, URLAttemptLowQuality, URLAttemptParseFailed, URLAttemptSearchFailed:
		return true
	default:
		return false
	}
}
