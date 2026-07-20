package docs

import (
	"strings"
)

func normalizeQueryPath(q string) string {
	q = strings.TrimSpace(q)
	q = strings.TrimPrefix(q, "docs/")
	q = strings.TrimPrefix(q, "./")
	q = strings.ReplaceAll(q, "\\", "/")
	q = strings.Trim(q, "/")
	if q == "" {
		return ""
	}
	if !strings.HasSuffix(strings.ToLower(q), ".md") {
		q += ".md"
	}
	return strings.ToLower(q)
}

func isDocsIndexQuery(q string) bool {
	q = strings.TrimSpace(strings.ToLower(q))
	q = strings.TrimPrefix(q, "docs/")
	q = strings.TrimSuffix(q, ".md")
	q = strings.Trim(q, "/")
	return q == "" || q == "readme"
}

func queryWordCount(q string) int {
	q = strings.TrimSpace(q)
	if q == "" {
		return 0
	}
	return len(strings.Fields(q))
}

func matchPaths(c *corpus, query string) []string {
	if isDocsIndexQuery(query) {
		if _, ok := c.articles[docsPortalPath]; ok {
			return []string{docsPortalPath}
		}
		return nil
	}
	norm := normalizeQueryPath(query)
	if norm == "" {
		return nil
	}
	var exact []string
	var partial []string
	for p := range c.articles {
		low := strings.ToLower(p)
		if low == norm {
			exact = append(exact, p)
			continue
		}
		if strings.HasSuffix(low, "/"+norm) || strings.HasSuffix(low, norm) {
			partial = append(partial, p)
		}
	}
	if len(exact) == 1 {
		return exact
	}
	if len(exact) > 1 {
		return pickReadmeRoot(exact)
	}
	if len(partial) == 1 {
		return partial
	}
	if len(partial) > 1 {
		return pickReadmeRoot(partial)
	}
	return nil
}

func pickReadmeRoot(paths []string) []string {
	for _, p := range paths {
		if p == docsPortalPath {
			return []string{docsPortalPath}
		}
	}
	if len(paths) == 1 {
		return paths
	}
	return paths
}
