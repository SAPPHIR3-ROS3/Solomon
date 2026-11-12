package main

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

var markdownHeading = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.*)$`)
var markdownList = regexp.MustCompile(`^( *)([-+*]|[0-9]+[.)])[ \t]+(.*)$`)
var markdownDivider = regexp.MustCompile(`^:?-{3,}:?$`)

func renderMarkdown(source string, width int) string {
	width = max(1, width)
	rows := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	var output []string
	for i := 0; i < len(rows); {
		row := rows[i]
		marker, language := markdownFence(row)
		if marker != "" {
			end := i + 1
			for end < len(rows) {
				closing, rest := markdownFence(rows[end])
				if len(closing) >= len(marker) && closing[0] == marker[0] && rest == "" {
					break
				}
				end++
			}
			if language != "" {
				output = append(output, highlightedWrappedLine(text(language, muted), width)...)
			}
			code := rows[i+1 : end]
			fields := strings.Fields(language)
			if len(fields) > 0 && (strings.EqualFold(fields[0], "go") || strings.EqualFold(fields[0], "golang")) {
				code = highlightGo(strings.Join(code, "\n"))
			}
			for _, line := range code {
				output = append(output, highlightedWrappedLine(line, width)...)
			}
			i = min(end+1, len(rows))
			continue
		}
		if i+1 < len(rows) && strings.Contains(row, "|") && markdownTableDivider(rows[i+1]) {
			end := i + 2
			for end < len(rows) && strings.TrimSpace(rows[end]) != "" && strings.Contains(rows[end], "|") {
				end++
			}
			table := append([]string{row}, rows[i+2:end]...)
			output = append(output, markdownTable(table, width)...)
			i = end
			continue
		}
		if heading := markdownHeading.FindStringSubmatch(row); heading != nil {
			row = bold(strings.TrimSpace(strings.TrimRight(heading[2], "#")), cyan)
		} else if item := markdownList.FindStringSubmatch(row); item != nil {
			marker := item[2]
			if marker == "-" || marker == "+" || marker == "*" {
				marker = "•"
			}
			prefix := item[1] + marker + " "
			indent := min(ansi.StringWidth(prefix), max(0, width-1))
			wrapped := highlightedWrappedLine(markdownInline(item[3]), max(1, width-indent))
			for n, line := range wrapped {
				lead := strings.Repeat(" ", indent)
				if n == 0 {
					lead = ansi.Truncate(prefix, indent, "")
				}
				output = append(output, lead+line)
			}
			i++
			continue
		} else {
			row = markdownInline(row)
		}
		output = append(output, highlightedWrappedLine(row, width)...)
		i++
	}
	return strings.Join(output, "\n")
}

func markdownFence(row string) (string, string) {
	trimmed := strings.TrimLeft(row, " ")
	if len(row)-len(trimmed) > 3 || len(trimmed) < 3 {
		return "", ""
	}
	char := trimmed[0]
	if char != 96 && char != '~' {
		return "", ""
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == char {
		n++
	}
	if n < 3 {
		return "", ""
	}
	return trimmed[:n], strings.TrimSpace(trimmed[n:])
}

func markdownInline(s string) string {
	var out strings.Builder
	for len(s) > 0 {
		if s[0] == '\\' && len(s) > 1 {
			out.WriteByte(s[1])
			s = s[2:]
			continue
		}
		if s[0] == 96 {
			n := 1
			for n < len(s) && s[n] == 96 {
				n++
			}
			delimiter := s[:n]
			if end := strings.Index(s[n:], delimiter); end >= 0 {
				out.WriteString(text(s[n:n+end], gold))
				s = s[n+end+n:]
				continue
			}
		}
		if strings.HasPrefix(s, "**") || strings.HasPrefix(s, "__") {
			if end := strings.Index(s[2:], s[:2]); end >= 0 {
				out.WriteString(bold(s[2:2+end], ink))
				s = s[4+end:]
				continue
			}
		}
		if s[0] == '[' {
			if middle := strings.Index(s, "]("); middle >= 0 {
				if end := strings.IndexByte(s[middle+2:], ')'); end >= 0 {
					label, target := s[1:middle], s[middle+2:middle+2+end]
					out.WriteString(text(label, cyan))
					if label != target {
						out.WriteString(text(" ("+target+")", muted))
					}
					s = s[middle+3+end:]
					continue
				}
			}
		}
		out.WriteByte(s[0])
		s = s[1:]
	}
	return out.String()
}

func markdownCells(row string) []string {
	row = strings.TrimSpace(row)
	row = strings.TrimPrefix(row, "|")
	row = strings.TrimSuffix(row, "|")
	var cells []string
	var cell strings.Builder
	code := false
	for i := 0; i < len(row); i++ {
		if row[i] == '\\' && i+1 < len(row) && row[i+1] == '|' {
			cell.WriteByte('|')
			i++
			continue
		}
		if row[i] == 96 {
			code = !code
		}
		if row[i] == '|' && !code {
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
		} else {
			cell.WriteByte(row[i])
		}
	}
	return append(cells, strings.TrimSpace(cell.String()))
}

func markdownTableDivider(row string) bool {
	cells := markdownCells(row)
	for _, cell := range cells {
		if !markdownDivider.MatchString(cell) {
			return false
		}
	}
	return len(cells) > 0
}

func markdownTable(rows []string, width int) []string {
	columns := len(markdownCells(rows[0]))
	if width < columns*2-1 {
		var result []string
		for _, row := range rows {
			result = append(result, highlightedWrappedLine(markdownInline(strings.Join(markdownCells(row), " | ")), width)...)
		}
		return result
	}
	widths := make([]int, columns)
	for i := range widths {
		widths[i] = 1
	}
	data := make([][]string, len(rows))
	for i, row := range rows {
		data[i] = markdownCells(row)
		for j := 0; j < min(columns, len(data[i])); j++ {
			data[i][j] = markdownInline(data[i][j])
			widths[j] = max(widths[j], ansi.StringWidth(data[i][j]))
		}
	}
	budget := width - (columns-1)*3
	if budget < columns {
		budget = columns
	}
	for {
		total, largest := 0, 0
		for j, size := range widths {
			total += size
			if size > widths[largest] {
				largest = j
			}
		}
		if total <= budget {
			break
		}
		widths[largest]--
	}
	separator := " | "
	if width < columns*4-3 {
		separator = "|"
	}
	var result []string
	for i, cells := range data {
		wrapped := make([][]string, columns)
		height := 1
		for j := range widths {
			value := ""
			if j < len(cells) {
				value = cells[j]
			}
			if i == 0 {
				value = bold(ansi.Strip(value), cyan)
			}
			wrapped[j] = highlightedWrappedLine(value, widths[j])
			height = max(height, len(wrapped[j]))
		}
		for line := 0; line < height; line++ {
			parts := make([]string, columns)
			for j := range parts {
				value := ""
				if line < len(wrapped[j]) {
					value = wrapped[j][line]
				}
				parts[j] = value + strings.Repeat(" ", max(0, widths[j]-ansi.StringWidth(value)))
			}
			result = append(result, strings.Join(parts, separator))
		}
		if i == 0 {
			parts := make([]string, columns)
			for j := range parts {
				parts[j] = strings.Repeat("─", widths[j])
			}
			result = append(result, text(strings.Join(parts, separator), muted))
		}
	}
	return result
}
