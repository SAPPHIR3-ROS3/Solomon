package tooling

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

const codeDisplayTabWidth = 4

var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true, "interface": true,
	"map": true, "package": true, "range": true, "return": true, "select": true,
	"struct": true, "switch": true, "type": true, "var": true,
}

func expandDisplayTabs(line string) string {
	if !strings.Contains(line, "\t") {
		return line
	}
	var b strings.Builder
	b.Grow(len(line) + strings.Count(line, "\t")*(codeDisplayTabWidth-1))
	col := 0
	for i := 0; i < len(line); {
		if line[i] == '\t' {
			pad := codeDisplayTabWidth - (col % codeDisplayTabWidth)
			if pad == 0 {
				pad = codeDisplayTabWidth
			}
			b.WriteString(strings.Repeat(" ", pad))
			col += pad
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(line[i:])
		b.WriteRune(r)
		col++
		i += size
	}
	return b.String()
}

func highlightGoLine(line string) string {
	if !termcolor.Enabled() || line == "" {
		return line
	}
	if i := strings.Index(line, "//"); i >= 0 {
		return highlightGoCode(line[:i]) + termcolor.GoComment(line[i:])
	}
	return highlightGoCode(line)
}

func highlightGoCode(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 32)
	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == '"' || r == '`' {
			end, chunk := readGoString(s[i:], r)
			b.WriteString(termcolor.GoString(chunk))
			i += end
			continue
		}
		if unicode.IsLetter(r) || r == '_' {
			j := i + size
			for j < len(s) {
				r2, sz := utf8.DecodeRuneInString(s[j:])
				if !unicode.IsLetter(r2) && !unicode.IsDigit(r2) && r2 != '_' {
					break
				}
				j += sz
			}
			word := s[i:j]
			switch {
			case goKeywords[word]:
				b.WriteString(termcolor.GoKeyword(word))
			case j < len(s) && s[j] == '(':
				b.WriteString(termcolor.GoFunction(word))
			default:
				b.WriteString(termcolor.GoPlain(word))
			}
			i = j
			continue
		}
		if unicode.IsDigit(r) {
			j := i + size
			for j < len(s) {
				r2, sz := utf8.DecodeRuneInString(s[j:])
				if !unicode.IsDigit(r2) && r2 != '.' && r2 != '_' {
					break
				}
				j += sz
			}
			b.WriteString(termcolor.GoNumber(s[i:j]))
			i = j
			continue
		}
		b.WriteString(termcolor.GoPlain(s[i : i+size]))
		i += size
	}
	return b.String()
}

func readGoString(s string, quote rune) (advance int, lit string) {
	if len(s) == 0 {
		return 0, ""
	}
	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == '\\' && i+size < len(s) {
			i += size + 1
			continue
		}
		i += size
		if r == quote {
			return i, s[:i]
		}
	}
	return len(s), s
}
