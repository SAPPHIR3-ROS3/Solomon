package replhl

import (
	"strings"
)

type inputKind int

const (
	inputPlain inputKind = iota
	inputShell
	inputSlash
)

func classifyBufferLine(lines [][]rune, row int, shellFirst bool) inputKind {
	if row < 0 || row >= len(lines) {
		return inputPlain
	}
	trimmed := strings.TrimSpace(string(lines[row]))
	if trimmed == "" {
		return inputPlain
	}
	if strings.HasPrefix(trimmed, "/") {
		return inputSlash
	}
	if shellFirst {
		if strings.HasPrefix(trimmed, "!") {
			return inputPlain
		}
		if lineLooksLikeChatText(lines[row]) {
			return inputPlain
		}
		return inputShell
	}
	if bufferShellMode(lines) {
		return inputShell
	}
	return inputPlain
}

func bufferShellMode(lines [][]rune) bool {
	if len(lines) == 0 {
		return false
	}
	trimmed := strings.TrimSpace(string(lines[0]))
	return strings.HasPrefix(trimmed, "!")
}

func shellLineText(lines [][]rune, row int, shellFirst bool) string {
	line := string(lines[row])
	if shellFirst {
		return strings.TrimSpace(line)
	}
	if row == 0 {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "!") {
			trimmed = strings.TrimLeft(trimmed[1:], " \t")
			return trimmed
		}
	}
	return line
}

func shellHighlightOffset(lines [][]rune, row int, shellFirst bool) int {
	if shellFirst || row > 0 {
		return 0
	}
	line := string(lines[row])
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, "!") {
		return 0
	}
	off := strings.Index(line, "!")
	if off < 0 {
		return 0
	}
	off++
	for off < len(line) && (line[off] == ' ' || line[off] == '\t') {
		off++
	}
	return off
}

func lineLooksLikeChatText(rs []rune) bool {
	hasWordApostrophe := false
	for i := range rs {
		if isWordApostrophe(rs, i) {
			hasWordApostrophe = true
			break
		}
	}
	if !hasWordApostrophe {
		return false
	}
	for _, ch := range rs {
		switch ch {
		case '|', '&', ';', '>', '<', '$', '*', '?', '[', ']', '#':
			return false
		}
	}
	return true
}
