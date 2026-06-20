package docs

import (
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

const (
	maxSnippetResults = 5
	maxSnippetChars   = 400
	maxFullArticleWords = 5
)

type SnippetResult struct {
	Path      string  `json:"path"`
	Heading   string  `json:"heading"`
	StartLine int     `json:"startLine"`
	EndLine   int     `json:"endLine"`
	Snippet   string  `json:"snippet"`
	Score     float64 `json:"score"`
}

type RetrievalResult struct {
	Mode    string           `json:"mode"`
	Query   string           `json:"query"`
	Path    string           `json:"path,omitempty"`
	Lines   int              `json:"lines,omitempty"`
	Content string           `json:"content,omitempty"`
	Results []SnippetResult  `json:"results,omitempty"`
}

type Options struct {
	MinNormalizedScore   float64
	FullArticleScore     float64
}

func Retrieve(query string, opts Options) (*RetrievalResult, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, fmt.Errorf("docsRetrieval: empty query")
	}
	c, err := loadCorpus()
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "docs corpus load failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return nil, err
	}
	if paths := matchPaths(c, q); len(paths) == 1 {
		return articleResult(q, c.articles[paths[0]]), nil
	}
	qterms := tokenizeSearchText(q)
	if len(qterms) == 0 {
		return nil, fmt.Errorf("docsRetrieval: query has no searchable terms")
	}
	texts := make([]string, len(c.chunks))
	for i, ch := range c.chunks {
		texts[i] = ch.searchText
	}
	corp := newBM25Corpus(texts)
	ranked := corp.rankedDocs(qterms)
	if len(ranked) == 0 {
		err := suggestError(q, c)
		logging.Log(logging.WARNING_LOG_LEVEL, "docs retrieval no match", logging.LogOptions{Params: map[string]any{"query": q, "err": err.Error()}})
		return nil, err
	}
	ceil := corp.scoreCeiling(qterms)
	topNorm := normalizeBM25Score(ranked[0].raw, ceil)
	if queryWordCount(q) <= maxFullArticleWords && topNorm >= opts.FullArticleScore {
		path := c.chunks[ranked[0].index].path
		return articleResult(q, c.articles[path]), nil
	}
	snippets, err := buildSnippets(c, ranked, ceil, opts.MinNormalizedScore)
	if err != nil {
		return nil, err
	}
	return &RetrievalResult{
		Mode:    "snippets",
		Query:   q,
		Results: snippets,
	}, nil
}

func articleResult(query string, a *article) *RetrievalResult {
	if a == nil {
		return nil
	}
	return &RetrievalResult{
		Mode:    "article",
		Query:   query,
		Path:    a.path,
		Lines:   a.lines,
		Content: a.content,
	}
}

func buildSnippets(c *corpus, ranked []scoredDoc, ceil, minNorm float64) ([]SnippetResult, error) {
	type merged struct {
		heading   string
		startLine int
		endLine   int
		parts     []string
		score     float64
	}
	byPath := map[string]*merged{}
	order := []string{}
	for _, sd := range ranked {
		norm := normalizeBM25Score(sd.raw, ceil)
		if minNorm > 0 && norm < minNorm {
			continue
		}
		ch := c.chunks[sd.index]
		m, ok := byPath[ch.path]
		if !ok {
			m = &merged{heading: ch.heading, startLine: ch.startLine, endLine: ch.endLine, score: norm}
			byPath[ch.path] = m
			order = append(order, ch.path)
		} else {
			if ch.startLine < m.startLine {
				m.startLine = ch.startLine
			}
			if ch.endLine > m.endLine {
				m.endLine = ch.endLine
			}
			if norm > m.score {
				m.score = norm
			}
			if m.heading == "" {
				m.heading = ch.heading
			}
		}
		m.parts = append(m.parts, strings.TrimSpace(ch.text))
		if len(order) > maxSnippetResults && len(byPath) > maxSnippetResults {
			break
		}
	}
	if len(byPath) == 0 {
		return nil, fmt.Errorf("docsRetrieval: no result reaches minimum normalized score %.4f", minNorm)
	}
	out := make([]SnippetResult, 0, maxSnippetResults)
	for _, path := range order {
		if len(out) >= maxSnippetResults {
			break
		}
		m := byPath[path]
		snippet := strings.Join(m.parts, "...")
		snippet = truncateSnippet(snippet, maxSnippetChars)
		out = append(out, SnippetResult{
			Path:      path,
			Heading:   m.heading,
			StartLine: m.startLine,
			EndLine:   m.endLine,
			Snippet:   snippet,
			Score:     m.score,
		})
	}
	return out, nil
}

func truncateSnippet(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return strings.TrimSpace(s[:max-3]) + "..."
}

func suggestError(query string, c *corpus) error {
	paths := allPaths(c)
	if len(paths) == 0 {
		return fmt.Errorf("docsRetrieval: no documentation indexed")
	}
	if len(paths) > 8 {
		paths = paths[:8]
	}
	return fmt.Errorf("docsRetrieval: no matching documentation for query %q (try paths such as: %s)", query, strings.Join(paths, ", "))
}

func FormatSlashPayload(query string, res *RetrievalResult) string {
	if res == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Use the following Solomon documentation for this turn.\n\nQuery: %q\n\n", query)
	switch res.Mode {
	case "article":
		fmt.Fprintf(&b, "Document: %s (%d lines)\n\n--- DOC START ---\n%s\n--- DOC END ---\n\nApply it to answer the user.", res.Path, res.Lines, res.Content)
	default:
		fmt.Fprintf(&b, "Mode: snippets\n\n")
		for i, r := range res.Results {
			if i > 0 {
				b.WriteString("\n\n")
			}
			fmt.Fprintf(&b, "### %s", r.Path)
			if r.Heading != "" {
				fmt.Fprintf(&b, " — %s", r.Heading)
			}
			fmt.Fprintf(&b, " (lines %d-%d, score %.2f)\n%s", r.StartLine, r.EndLine, r.Score, r.Snippet)
		}
		b.WriteString("\n\nUse these excerpts to answer the user.")
	}
	return b.String()
}
