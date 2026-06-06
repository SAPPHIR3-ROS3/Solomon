package atmention

import (
	"path/filepath"
	"sort"
	"strings"
)

const MaxPickerResults = 10

type Entry struct {
	RelPath string
	IsDir   bool
}

type scoredEntry struct {
	entry Entry
	score int
}

func normalizeRel(p string) string {
	p = filepath.ToSlash(strings.TrimSpace(p))
	p = strings.TrimPrefix(p, "./")
	return p
}

func pathHasSuffix(path, suffix string) bool {
	path = normalizeRel(path)
	suffix = normalizeRel(suffix)
	if path == suffix {
		return true
	}
	return strings.HasSuffix(path, "/"+suffix)
}

func ShortTag(relPath string, all []Entry) string {
	relPath = normalizeRel(relPath)
	parts := strings.Split(relPath, "/")
	for i := 0; i < len(parts); i++ {
		suffix := strings.Join(parts[i:], "/")
		if suffixMatchCount(suffix, all) == 1 {
			return suffix
		}
	}
	return relPath
}

func suffixMatchCount(suffix string, all []Entry) int {
	n := 0
	for _, e := range all {
		if pathHasSuffix(e.RelPath, suffix) {
			n++
		}
	}
	return n
}

func ResolveTag(tag string, all []Entry) (Entry, bool) {
	tag = normalizeRel(tag)
	var matches []Entry
	for _, e := range all {
		if pathHasSuffix(e.RelPath, tag) {
			matches = append(matches, e)
		}
	}
	if len(matches) != 1 {
		return Entry{}, false
	}
	return matches[0], true
}

func entryMatchScore(query, rel string) (int, bool) {
	query = strings.TrimSpace(query)
	if query == "" {
		return 0, false
	}
	rel = normalizeRel(rel)
	base := filepath.Base(rel)
	if strings.HasPrefix(base, query) {
		return 0, true
	}
	for _, part := range strings.Split(rel, "/") {
		if part != "" && strings.HasPrefix(part, query) {
			return 1, true
		}
	}
	if len(query) >= 3 {
		if strings.Contains(base, query) {
			return 2, true
		}
		if strings.Contains(rel, query) {
			return 3, true
		}
	}
	return 0, false
}

func MatchQuery(query string, all []Entry, limit int) []Entry {
	query = normalizeRel(query)
	if query == "" {
		return nil
	}
	if limit <= 0 {
		limit = MaxPickerResults
	}
	var scored []scoredEntry
	for _, e := range all {
		if score, ok := entryMatchScore(query, e.RelPath); ok {
			scored = append(scored, scoredEntry{entry: e, score: score})
		}
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score < scored[j].score
		}
		return normalizeRel(scored[i].entry.RelPath) < normalizeRel(scored[j].entry.RelPath)
	})
	out := make([]Entry, 0, limit)
	for _, s := range scored {
		out = append(out, s.entry)
		if len(out) >= limit {
			break
		}
	}
	return out
}
