package html

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldhtml "github.com/yuin/goldmark/renderer/html"
)

type Source struct {
	URL   string
	Title string
}

type Stats struct {
	DurationSecs float64
	Rounds       int
	Queries      int
	URLs         int
	Findings     int
	SearchEngine string
	Model        string
}

type Input struct {
	Title    string
	Question string
	Markdown string
	Findings []Source
	Stats    Stats
}

var mdRenderer = goldmark.New(
	goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.TaskList),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(goldhtml.WithUnsafe()),
)

func Render(in Input) (string, error) {
	bodyHTML, err := markdownToHTML(in.Markdown)
	if err != nil {
		return "", err
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = strings.TrimSpace(in.Question)
	}
	if title == "" {
		title = "Research Report"
	}
	toc := buildTOC(bodyHTML)
	sources := renderSources(in.Findings)
	stats := renderStats(in.Stats)
	date := time.Now().UTC().Format("January 2, 2006")
	return fmt.Sprintf(pageTemplate,
		html.EscapeString(title),
		css(),
		html.EscapeString(title),
		html.EscapeString(in.Question),
		date,
		toc,
		bodyHTML,
		sources,
		stats,
	), nil
}

func markdownToHTML(md string) (string, error) {
	var buf bytes.Buffer
	if err := mdRenderer.Convert([]byte(md), &buf); err != nil {
		return "", err
	}
	return sanitizeHTML(buf.String()), nil
}

var headingRe = regexp.MustCompile(`<h([2-4]) id="([^"]+)">([^<]+)</h[2-4]>`)

func buildTOC(body string) string {
	matches := headingRe.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<nav class="toc"><h2>Contents</h2><ul>`)
	for _, m := range matches {
		if len(m) < 4 {
			continue
		}
		level := m[1]
		id := m[2]
		text := m[3]
		cls := "toc-l" + level
		b.WriteString(`<li class="` + cls + `"><a href="#` + html.EscapeString(id) + `">` + text + `</a></li>`)
	}
	b.WriteString(`</ul></nav>`)
	return b.String()
}

func renderSources(sources []Source) string {
	if len(sources) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<section class="sources"><details open><summary>Sources (` + fmt.Sprint(len(sources)) + `)</summary><ol>`)
	for _, s := range sources {
		title := strings.TrimSpace(s.Title)
		if title == "" {
			title = s.URL
		}
		b.WriteString(`<li><a href="` + html.EscapeString(s.URL) + `" target="_blank" rel="noopener">` + html.EscapeString(title) + `</a></li>`)
	}
	b.WriteString(`</ol></details></section>`)
	return b.String()
}

func renderStats(s Stats) string {
	parts := []string{}
	if s.DurationSecs > 0 {
		parts = append(parts, fmt.Sprintf("Duration: %.1fs", s.DurationSecs))
	}
	if s.Rounds > 0 {
		parts = append(parts, fmt.Sprintf("Rounds: %d", s.Rounds))
	}
	if s.Queries > 0 {
		parts = append(parts, fmt.Sprintf("Queries: %d", s.Queries))
	}
	if s.URLs > 0 {
		parts = append(parts, fmt.Sprintf("URLs: %d", s.URLs))
	}
	if s.Findings > 0 {
		parts = append(parts, fmt.Sprintf("Findings: %d", s.Findings))
	}
	if s.SearchEngine != "" {
		parts = append(parts, "Search: "+html.EscapeString(s.SearchEngine))
	}
	if s.Model != "" {
		parts = append(parts, "Model: "+html.EscapeString(s.Model))
	}
	if len(parts) == 0 {
		return ""
	}
	return `<footer class="stats"><p>` + strings.Join(parts, " · ") + `</p></footer>`
}

const pageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<style>%s</style>
</head>
<body>
<header class="hero">
<h1>%s</h1>
<p class="question">%s</p>
<p class="date">%s</p>
</header>
<main>
%s
<article class="report">%s</article>
%s
%s
</main>
</body>
</html>`
