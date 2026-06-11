package plan

import (
	"strings"
)

type Sections struct {
	Goal    string
	Context string
	Design  string
	Todo    TodoSection
}

type TodoSection struct {
	Rules     string
	Mermaid   string
	Checklist []TodoItem
	RawBody   string
}

func ParseSections(body []byte) Sections {
	s := string(body)
	sec := Sections{}
	sec.Goal = sectionBody(s, "# Goal")
	sec.Context = sectionBody(s, "## Context")
	sec.Design = sectionBody(s, "## Design")
	if raw := sectionBody(s, "## Todo"); raw != "" || strings.Contains(s, "## Todo") {
		sec.Todo = parseTodoSection(raw)
	}
	return sec
}

func sectionBody(s, heading string) string {
	lines := strings.Split(s, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == heading {
			start = i + 1
			break
		}
	}
	if start < 0 || start >= len(lines) {
		return ""
	}
	var buf []string
	for i := start; i < len(lines); i++ {
		trim := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trim, "## ") || (heading != "# Goal" && strings.HasPrefix(trim, "# Goal")) {
			break
		}
		if heading == "# Goal" && strings.HasPrefix(trim, "## ") {
			break
		}
		buf = append(buf, lines[i])
	}
	return strings.TrimSpace(strings.Join(buf, "\n"))
}

func parseTodoSection(raw string) TodoSection {
	ts := TodoSection{RawBody: raw}
	if raw == "" {
		return ts
	}
	lines := strings.Split(raw, "\n")
	mermaidStart := -1
	mermaidEnd := -1
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "```mermaid" {
			mermaidStart = i
			continue
		}
		if mermaidStart >= 0 && mermaidEnd < 0 && trim == "```" {
			mermaidEnd = i
		}
	}
	checkStart := len(lines)
	for i := len(lines) - 1; i >= 0; i-- {
		if todoLineRE.MatchString(strings.TrimRight(lines[i], "\r")) {
			checkStart = i
		} else if checkStart < len(lines) {
			break
		}
	}
	var rulesLines []string
	var checklistLines []string
	for i, line := range lines {
		if mermaidStart >= 0 && i >= mermaidStart && i <= mermaidEnd {
			if i > mermaidStart && i < mermaidEnd {
				ts.Mermaid += line + "\n"
			}
			continue
		}
		if i >= checkStart && todoLineRE.MatchString(strings.TrimRight(line, "\r")) {
			checklistLines = append(checklistLines, line)
			continue
		}
		if mermaidEnd >= 0 && i > mermaidEnd && i < checkStart {
			rulesLines = append(rulesLines, line)
			continue
		}
		if mermaidStart < 0 && i < checkStart {
			rulesLines = append(rulesLines, line)
		}
	}
	ts.Rules = strings.TrimSpace(strings.Join(rulesLines, "\n"))
	ts.Mermaid = strings.TrimSpace(ts.Mermaid)
	ts.Checklist = ParseChecklist(checklistLines)
	return ts
}

func SkeletonBody(goal string) []byte {
	g := strings.TrimSpace(goal)
	return []byte("# Goal\n\n" + g + "\n\n## Context\n\n## Design\n\n## Todo\n\n")
}

func RefreshStatus(body []byte) ([]byte, Meta, error) {
	meta, rest, err := ParseDocument(body)
	if err != nil {
		return nil, Meta{}, err
	}
	sec := ParseSections(rest)
	meta.Status = StatusFromItems(sec.Todo.Checklist)
	out, err := WriteDocument(meta, rest)
	return out, meta, err
}
