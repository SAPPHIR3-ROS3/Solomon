package test

import (
	"strings"
	"testing"

	researchhtml "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/research/html"
)

func TestHTMLRenderContainsTLDR(t *testing.T) {
	t.Parallel()
	md := "# Main Report\n\n## Section\n\nDetailed analysis here.\n\n## TL;DR\n\nExecutive summary at the end."
	out, err := researchhtml.Render(researchhtml.Input{
		Title:    "Test",
		Question: "Test question?",
		Markdown: md,
		Findings: []researchhtml.Source{{URL: "https://example.com", Title: "Example"}},
		Stats:    researchhtml.Stats{Rounds: 2, URLs: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<!DOCTYPE html>") {
		t.Fatal("missing doctype")
	}
	if !strings.Contains(out, "TL;DR") {
		t.Fatal("missing TLDR in html")
	}
	if !strings.Contains(out, "prefers-color-scheme") {
		t.Fatal("missing theme css")
	}
	if !strings.Contains(out, "<style>\n:root {") {
		t.Fatal("css must be inside style tag")
	}
	if !strings.Contains(out, "<title>Test</title>") {
		t.Fatal("title must be in title tag")
	}
	if strings.Contains(out, "<title>\n:root") {
		t.Fatal("css leaked into title tag")
	}
	if !strings.Contains(out, "https://example.com") {
		t.Fatal("missing source link")
	}
}

func TestHasTLDRSection(t *testing.T) {
	t.Parallel()
	if !researchhtml.HasTLDRSection("## TL;DR\n\nfoo") {
		t.Fatal("expected tldr section")
	}
}
