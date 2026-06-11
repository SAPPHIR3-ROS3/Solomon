package plan

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var todoLineRE = regexp.MustCompile(`^-\s+\[([ xX])\]\s+(.+?)\s+-\s+([0-9a-f]{40})\s*$`)

type TodoItem struct {
	SHA     string
	Text    string
	Checked bool
	Line    string
}

func TodoSHA(text string) string {
	h := sha1.Sum([]byte(strings.TrimSpace(text)))
	return hex.EncodeToString(h[:])
}

func FormatTodoLine(text string, checked bool) string {
	text = strings.TrimSpace(text)
	mark := " "
	if checked {
		mark = "x"
	}
	return fmt.Sprintf("- [%s] %s - %s", mark, text, TodoSHA(text))
}

func ParseChecklist(lines []string) []TodoItem {
	var items []TodoItem
	for _, line := range lines {
		m := todoLineRE.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		items = append(items, TodoItem{
			SHA:     m[3],
			Text:    strings.TrimSpace(m[2]),
			Checked: strings.EqualFold(m[1], "x"),
			Line:    line,
		})
	}
	return items
}

func OpenTodos(items []TodoItem) []TodoItem {
	var open []TodoItem
	for _, it := range items {
		if !it.Checked {
			open = append(open, it)
		}
	}
	return open
}

func StatusFromItems(items []TodoItem) string {
	total := len(items)
	checked := 0
	for _, it := range items {
		if it.Checked {
			checked++
		}
	}
	return ComputeStatus(total, checked)
}

func ReplaceTodoChecked(body string, sha string) (string, bool, error) {
	lines := strings.Split(body, "\n")
	found := false
	for i, line := range lines {
		m := todoLineRE.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil || m[3] != sha {
			continue
		}
		lines[i] = FormatTodoLine(m[2], true)
		found = true
		break
	}
	if !found {
		return body, false, nil
	}
	return strings.Join(lines, "\n"), true, nil
}

func RemoveTodoLine(body string, sha string) (string, bool, error) {
	lines := strings.Split(body, "\n")
	var out []string
	found := false
	for _, line := range lines {
		m := todoLineRE.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m != nil && m[3] == sha {
			found = true
			continue
		}
		out = append(out, line)
	}
	if !found {
		return body, false, nil
	}
	return strings.Join(out, "\n"), true, nil
}

func AppendTodoLine(body string, text string) string {
	line := FormatTodoLine(text, false)
	body = strings.TrimRight(body, "\n")
	if body == "" {
		return line + "\n"
	}
	return body + "\n" + line + "\n"
}
