package tooling

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"golang.org/x/term"
)

var orchestrateGutterRE = regexp.MustCompile(`^\s*\d+ `)

func WriteToolDisplayLinesWithWidth(out io.Writer, cpSeq int, branchKey string, lines []string, width int) {
	writeToolDisplayLinesWithPrefixes(out, checkpoint.FormatCheckpointPrefix(cpSeq, branchKey), checkpoint.FormatCheckpointContinuationPlain(cpSeq, branchKey), lines, width)
}

func writeToolDisplayLines(out io.Writer, cpSeq int, branchKey string, lines []string, termW int) {
	writeToolDisplayLinesWithPrefixes(out, checkpoint.FormatCheckpointPrefix(cpSeq, branchKey), checkpoint.FormatCheckpointContinuationPlain(cpSeq, branchKey), lines, termW)
}

// WriteToolDisplayLinesWithPrefixes renders lines with explicit first-line
// and continuation prefixes while retaining terminal-aware soft wrapping.
func WriteToolDisplayLinesWithPrefixes(out io.Writer, firstPrefixPlain, continuationPrefixPlain string, lines []string) {
	writeToolDisplayLinesWithPrefixes(out, firstPrefixPlain, continuationPrefixPlain, lines, terminalWidthForWriter(out))
}

func WriteToolDisplayLinesWithPrefixesAndWidth(out io.Writer, firstPrefixPlain, continuationPrefixPlain string, lines []string, width int) {
	writeToolDisplayLinesWithPrefixes(out, firstPrefixPlain, continuationPrefixPlain, lines, width)
}

func writeToolDisplayLinesWithPrefixes(out io.Writer, firstPrefixPlain, continuationPrefixPlain string, lines []string, termW int) {
	first := true
	firstPrefix := firstPrefixPlain
	cont := termcolor.WrapUserReadline(continuationPrefixPlain)
	for _, line := range lines {
		for _, part := range strings.Split(line, "\n") {
			prefix := firstPrefix
			prefixCells := visibleDisplayCells(firstPrefix)
			if !first {
				prefix = cont
				prefixCells = visibleDisplayCells(cont)
			}
			chunks := wrapToolDisplayPart(part, termW, prefixCells)
			for i, chunk := range chunks {
				rowPrefix := prefix
				if i > 0 {
					rowPrefix = cont
				}
				fmt.Fprintf(out, "%s%s\n", rowPrefix, chunk)
			}
			first = false
		}
	}
}

func terminalWidthForWriter(out io.Writer) int {
	fd, ok := terminalFDForWriter(out)
	if !ok || !term.IsTerminal(fd) {
		return 0
	}
	w, _, err := term.GetSize(fd)
	if err != nil || w < 20 {
		return 0
	}
	return w
}

func terminalFDForWriter(out io.Writer) (int, bool) {
	if out == nil {
		return 0, false
	}
	if f, ok := out.(interface{ TerminalFD() (uintptr, bool) }); ok {
		fd, ok := f.TerminalFD()
		return int(fd), ok
	}
	if f, ok := out.(interface{ Fd() uintptr }); ok {
		return int(f.Fd()), true
	}
	return 0, false
}

func wrapToolDisplayPart(part string, termW, prefixCells int) []string {
	if termW <= 0 {
		return []string{part}
	}
	if termcolor.IsEditLineDisplay(part) {
		return wrapEditLinePart(part, termW, prefixCells)
	}
	if gutterLen, ok := orchestrateGutterLen(part); ok {
		return wrapOrchestrateCodePart(part, termW, prefixCells, gutterLen)
	}
	return wrapStyledToolLinePart(part, termW, prefixCells)
}

func wrapStyledToolLinePart(part string, termW, prefixCells int) []string {
	budget := termW - prefixCells
	if budget < 8 {
		return []string{part}
	}
	plain := termcolor.Plain(part)
	if visibleDisplayCells(plain) <= budget {
		return []string{part}
	}
	chunks := wrapPlainAtBudget(plain, budget)
	out := make([]string, len(chunks))
	offsetRunes := 0
	for i, c := range chunks {
		n := utf8.RuneCountInString(c)
		out[i] = splitStyledRange(part, offsetRunes, offsetRunes+n)
		offsetRunes += n
	}
	return out
}

func wrapEditLinePart(part string, termW, prefixCells int) []string {
	budget := termW - prefixCells
	if budget < 8 {
		return []string{part}
	}
	plain := termcolor.Plain(part)
	if visibleDisplayCells(plain) <= budget {
		return []string{part}
	}
	chunks := wrapPlainAtBudget(plain, budget)
	out := make([]string, len(chunks))
	for i, c := range chunks {
		out[i] = termcolor.RewrapEditLineLike(part, c)
	}
	return out
}

func orchestrateGutterLen(part string) (int, bool) {
	plain := termcolor.Plain(part)
	if plain == "Code" || strings.Contains(plain, orchestrateTruncatedMarker) {
		return 0, false
	}
	loc := orchestrateGutterRE.FindStringIndex(plain)
	if loc == nil {
		return 0, false
	}
	return loc[1], true
}

func wrapOrchestrateCodePart(part string, termW, prefixCells, gutterLen int) []string {
	gutterStyled, _ := splitStyledAtPlainOffset(part, gutterLen)
	gutterCells := visibleDisplayCells(termcolor.Plain(gutterStyled))
	codePlain := termcolor.Plain(part)[gutterLen:]
	budget := termW - prefixCells - gutterCells
	if budget < 8 {
		return []string{part}
	}
	if visibleDisplayCells(codePlain) <= budget {
		return []string{part}
	}
	codeStyled := highlightGoLine(codePlain)
	chunks := wrapPlainAtBudgetHard(codePlain, budget)
	blankGutter := strings.Repeat(" ", gutterCells)
	out := make([]string, len(chunks))
	offsetRunes := 0
	for i, c := range chunks {
		n := utf8.RuneCountInString(c)
		styledChunk := splitStyledRange(codeStyled, offsetRunes, offsetRunes+n)
		offsetRunes += n
		if i == 0 {
			out[i] = gutterStyled + styledChunk
		} else {
			out[i] = blankGutter + styledChunk
		}
	}
	return out
}

func splitStyledRange(styled string, start, end int) string {
	return styledPlainSlice(styled, start, end)
}

func styledPlainSlice(styled string, start, end int) string {
	if start < 0 || end <= start {
		return ""
	}
	var out strings.Builder
	plainIdx := 0
	i := 0
	var active string
	collecting := false
	for i < len(styled) {
		if styled[i] == '\x1b' {
			endSeq := strings.IndexByte(styled[i+1:], 'm')
			if endSeq < 0 {
				break
			}
			seq := styled[i : i+endSeq+2]
			if ansiReset(seq) {
				active = ""
			} else {
				active = seq
			}
			if collecting {
				out.WriteString(seq)
			}
			i += endSeq + 2
			continue
		}
		if plainIdx >= end {
			break
		}
		_, size := utf8.DecodeRuneInString(styled[i:])
		if size == 0 {
			break
		}
		if plainIdx == start {
			collecting = true
			if start > 0 && active != "" {
				out.WriteString(active)
			}
		}
		if collecting {
			out.WriteString(styled[i : i+size])
		}
		plainIdx++
		i += size
	}
	if collecting && active != "" && termcolor.Enabled() {
		out.WriteString("\x1b[0m")
	}
	return out.String()
}

func ansiReset(seq string) bool {
	return seq == "\x1b[0m" || seq == "\x1b[m"
}

func SplitStyledRangeForTest(styled string, start, end int) string {
	return splitStyledRange(styled, start, end)
}

func splitStyledAtPlainOffset(styled string, plainOffset int) (before, after string) {
	if plainOffset <= 0 {
		return "", styled
	}
	plainIdx := 0
	i := 0
	for i < len(styled) {
		if styled[i] == '\x1b' {
			end := strings.IndexByte(styled[i+1:], 'm')
			if end < 0 {
				break
			}
			i += end + 2
			continue
		}
		if plainIdx == plainOffset {
			return styled[:i], styled[i:]
		}
		_, size := utf8.DecodeRuneInString(styled[i:])
		if size == 0 {
			break
		}
		plainIdx++
		i += size
	}
	return styled, ""
}

func wrapPlainAtBudget(text string, budget int) []string {
	text = strings.TrimRight(text, " \t")
	if text == "" {
		return []string{" "}
	}
	var rows []string
	rest := text
	for len(rest) > 0 {
		if visibleDisplayCells(rest) <= budget {
			rows = append(rows, rest)
			break
		}
		cut := plainBreakIndex(rest, budget)
		if cut <= 0 {
			cut = 1
		}
		rows = append(rows, rest[:cut])
		rest = strings.TrimLeft(rest[cut:], " ")
	}
	return rows
}

func wrapPlainAtBudgetHard(text string, budget int) []string {
	text = strings.TrimRight(text, " \t")
	if text == "" {
		return []string{" "}
	}
	var rows []string
	rest := text
	for len(rest) > 0 {
		if visibleDisplayCells(rest) <= budget {
			rows = append(rows, rest)
			break
		}
		cut := plainBreakIndexHard(rest, budget)
		if cut <= 0 {
			cut = 1
		}
		rows = append(rows, rest[:cut])
		rest = rest[cut:]
	}
	return rows
}

func plainBreakIndexHard(text string, budget int) int {
	if visibleDisplayCells(text) <= budget {
		return len(text)
	}
	width := 0
	pos := 0
	for i, r := range text {
		cw := runeDisplayWidth(r)
		if width+cw > budget {
			if pos == 0 {
				return i + len(string(r))
			}
			return pos
		}
		width += cw
		pos = i + len(string(r))
	}
	return len(text)
}

func plainBreakIndex(text string, budget int) int {
	if visibleDisplayCells(text) <= budget {
		return len(text)
	}
	width := 0
	lastSpaceEnd := 0
	pos := 0
	for i, r := range text {
		cw := runeDisplayWidth(r)
		if width+cw > budget {
			if lastSpaceEnd > 0 {
				return lastSpaceEnd
			}
			if pos == 0 {
				return i + len(string(r))
			}
			return pos
		}
		if r == ' ' {
			lastSpaceEnd = i + len(string(r))
		}
		width += cw
		pos = i + len(string(r))
	}
	return len(text)
}

func visibleDisplayCells(s string) int {
	return displayCells(termcolor.Plain(s))
}

func displayCells(s string) int {
	n := 0
	for _, r := range s {
		n += runeDisplayWidth(r)
	}
	return n
}

func runeDisplayWidth(r rune) int {
	switch {
	case r == 0 || r < 0x20:
		return 0
	case (r >= 0x1100 && r <= 0x115F) || (r >= 0x2329 && r <= 0x232A):
		return 2
	case r >= 0x2E80 && r <= 0x303E:
		return 2
	case r >= 0x3040 && r <= 0x3247:
		return 2
	case r >= 0x3250 && r <= 0x4DBF:
		return 2
	case r >= 0x4E00 && r <= 0x9FFF:
		return 2
	case r >= 0xA000 && r <= 0xA4C6:
		return 2
	case r >= 0xAC00 && r <= 0xD7A3:
		return 2
	case r >= 0xF900 && r <= 0xFAFF:
		return 2
	case r >= 0xFE10 && r <= 0xFE19:
		return 2
	case r >= 0xFE30 && r <= 0xFE6F:
		return 2
	case r >= 0xFF00 && r <= 0xFF60:
		return 2
	case r >= 0xFFE0 && r <= 0xFFE6:
		return 2
	case r >= 0x20000 && r <= 0x2FFFD:
		return 2
	case r >= 0x30000 && r <= 0x3FFFD:
		return 2
	default:
		return 1
	}
}
