package test

import (
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/research"
)

func TestParseJSONArray(t *testing.T) {
	t.Parallel()
	got := research.ParseJSONArrayForTest(`["one", "two", "three"]`)
	if len(got) != 3 || got[0] != "one" {
		t.Fatalf("parse array: %#v", got)
	}
}

func TestSlugFromQuery(t *testing.T) {
	t.Parallel()
	s := research.SlugFromQuery("Voglio sapere qual è il miglior linguaggio")
	if strings.Contains(s, "---") {
		t.Fatalf("slug has collapsed dashes expected: %q", s)
	}
	if !strings.HasPrefix(s, "voglio-sapere") {
		t.Fatalf("slug: %q", s)
	}
}

func TestTitleFromQuery(t *testing.T) {
	t.Parallel()
	title := research.TitleFromQuery("Voglio sapere qual è il miglior linguaggio")
	if !strings.Contains(title, "Voglio sapere") {
		t.Fatalf("title: %q", title)
	}
}

func TestIsLowQuality(t *testing.T) {
	t.Parallel()
	if !research.IsLowQualityForTest("content is insufficient for the goal") {
		t.Fatal("expected low quality")
	}
	if research.IsLowQualityForTest("Strong evidence about market growth in 2024.") {
		t.Fatal("expected useful summary")
	}
}

func TestAppendTLDRSection(t *testing.T) {
	t.Parallel()
	out := research.AppendTLDRSection("# Report\n\nBody.", "Short synthesis.")
	if !strings.Contains(out, "## TL;DR") {
		t.Fatalf("missing TLDR: %q", out)
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "Short synthesis.") {
		t.Fatalf("TLDR not at end: %q", out)
	}
}

func TestShouldStopFromLLM(t *testing.T) {
	t.Parallel()
	if !research.ShouldStopFromLLMForTest("YES — enough coverage") {
		t.Fatal("expected stop")
	}
	if research.ShouldStopFromLLMForTest("NO — need more data") {
		t.Fatal("expected continue")
	}
}

func TestFallbackQueries(t *testing.T) {
	t.Parallel()
	got := research.FallbackQueriesForTest("best GGUF 8GB VRAM", 1)
	if len(got) < 2 || got[0] != "best GGUF 8GB VRAM" {
		t.Fatalf("round 1: %#v", got)
	}
	got = research.FallbackQueriesForTest("best GGUF 8GB VRAM", 2)
	if len(got) != 2 {
		t.Fatalf("round 2: %#v", got)
	}
}

func TestFormatJobStatsLine(t *testing.T) {
	t.Parallel()
	running := research.FormatJobStatsLine(research.JobRecord{Status: research.StatusRunning})
	if running != "0 queries · 0 urls · 0 findings" {
		t.Fatalf("running: %q", running)
	}
	done := research.FormatJobStatsLine(research.JobRecord{
		Status: research.StatusDone,
		Stats:  research.JobStats{Queries: 4, URLs: 12, Findings: 8},
	})
	if done != "4 queries · 12 urls · 8 findings" {
		t.Fatalf("done: %q", done)
	}
	withFailures := research.FormatJobStatsLine(research.JobRecord{
		Status: research.StatusRunning,
		Stats: research.JobStats{
			Queries: 3, URLs: 10, Findings: 1,
			URLFetchFailed: 2, URLLLMFailed: 5, SearchFailures: 1,
		},
	})
	if !strings.Contains(withFailures, "2 fetch, 5 llm, 1 search failed") {
		t.Fatalf("failures: %q", withFailures)
	}
	empty := research.FormatJobStatsLine(research.JobRecord{Status: research.StatusFailed})
	if empty != "" {
		t.Fatalf("failed empty stats: %q", empty)
	}
	paused := research.FormatJobStatsLine(research.JobRecord{Status: research.StatusPaused})
	if paused != "0 queries · 0 urls · 0 findings" {
		t.Fatalf("paused: %q", paused)
	}
}

func TestURLAttemptTracking(t *testing.T) {
	t.Parallel()
	rec := &research.JobRecord{}
	research.ApplyURLAttemptForTest(rec, research.ProgressEvent{
		URL: "https://example.com", Message: research.URLAttemptFetchFailed, Title: "timeout",
	})
	if rec.Stats.URLFetchFailed != 1 {
		t.Fatalf("fetch failed count: %d", rec.Stats.URLFetchFailed)
	}
	if len(rec.URLAttempts) != 1 {
		t.Fatalf("url attempts: %d", len(rec.URLAttempts))
	}
	line := research.FormatURLAttemptLine(*rec, research.ProgressEvent{
		URL: "https://example.com", Message: research.URLAttemptLLMFailed, Title: "model timeout",
	})
	if !strings.Contains(line, "LLM extract failed") || !strings.Contains(line, "https://example.com") {
		t.Fatalf("line: %q", line)
	}
	summary := research.FormatURLFailureSummary(rec.Stats)
	if summary != "1 fetch failed" {
		t.Fatalf("summary: %q", summary)
	}
	research.ApplyURLAttemptForTest(rec, research.ProgressEvent{
		Message: research.URLAttemptSearchFailed, QueryPreview: "best llm", Title: "no results",
	})
	if rec.Stats.SearchFailures != 1 {
		t.Fatalf("search failures: %d", rec.Stats.SearchFailures)
	}
	searchLine := research.FormatURLAttemptLine(*rec, research.ProgressEvent{
		Message: research.URLAttemptSearchFailed, QueryPreview: "best llm", Title: "no results",
	})
	if !strings.Contains(searchLine, "search failed") {
		t.Fatalf("search line: %q", searchLine)
	}
}

func TestPauseLLMError(t *testing.T) {
	t.Parallel()
	err := research.PauseLLMErrorForTest("timeout")
	if !research.IsPausedLLMForTest(err) {
		t.Fatalf("expected paused LLM error: %v", err)
	}
}

func TestFormatResearchError(t *testing.T) {
	t.Parallel()
	raw := `POST "http://localhost:1234/v1/chat/completions": 400 Bad Request { "message": "No models loaded. Please load a model.", "type": "invalid_request_error" }`
	got := research.FormatResearchErrorForTest(raw)
	want := "No models loaded. Please load a model."
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
