package shelllex

import (
	"strings"
)

type Word struct {
	Start int
	End   int
	Text  string
}

type Segment struct {
	Start int
	End   int
	Words []Word
}

func QuoteStateAt(shell []rune, cursor int) (inQuote bool, quote rune) {
	inQ := false
	var q rune
	for i := 0; i < cursor; i++ {
		ch := shell[i]
		if inQ {
			if ch == q && !IsEscapedAt(shell, i) {
				inQ = false
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			inQ = true
			q = ch
		}
	}
	return inQ, q
}

func IsEscapedAt(shell []rune, i int) bool {
	return i > 0 && shell[i-1] == '\\'
}

func ShellTokenBounds(shell []rune, cursor int) (start, end int) {
	if cursor > 0 && (shell[cursor-1] == ' ' || shell[cursor-1] == '\t') {
		start = cursor
		for start < len(shell) && (shell[start] == ' ' || shell[start] == '\t') {
			start++
		}
		return start, cursor
	}
	end = cursor
	for end > 0 && (shell[end-1] == ' ' || shell[end-1] == '\t') {
		end--
	}
	if end == 0 {
		return 0, 0
	}
	if inQuote, q := QuoteStateAt(shell, cursor); inQuote {
		for i := cursor - 1; i >= 0; i-- {
			if shell[i] == q && !IsEscapedAt(shell, i) {
				return i + 1, cursor
			}
		}
		return 0, cursor
	}
	start = end - 1
	for start >= 0 {
		ch := shell[start]
		if ch == ' ' || ch == '\t' {
			if IsEscapedAt(shell, start) {
				start--
				continue
			}
			break
		}
		if IsShellOpAt(shell, start) {
			break
		}
		start--
	}
	start++
	if start < 0 {
		start = 0
	}
	return start, cursor
}

func SegmentWordsAtCursor(shell []rune, cursor int) []Word {
	segStart := segmentStart(shell, cursor)
	var words []Word
	i := segStart
	for i < cursor {
		for i < cursor && (shell[i] == ' ' || shell[i] == '\t') {
			i++
		}
		if i >= cursor {
			break
		}
		if IsShellOpAt(shell, i) {
			i = SkipShellOp(shell, i)
			continue
		}
		wStart := i
		i = scanWordEnd(shell, i, cursor)
		words = append(words, Word{Start: wStart, End: i, Text: string(shell[wStart:i])})
	}
	return words
}

func Segments(line []rune) []Segment {
	if len(line) == 0 {
		return nil
	}
	var segs []Segment
	segStart := 0
	i := 0
	for i <= len(line) {
		if i == len(line) || isSegmentBreak(line, i) {
			words := wordsInRange(line, segStart, i)
			if len(words) > 0 || segStart < i {
				segs = append(segs, Segment{Start: segStart, End: i, Words: words})
			}
			if i >= len(line) {
				break
			}
			i = SkipShellOp(line, i)
			segStart = i
			continue
		}
		i++
	}
	return segs
}

func isSegmentBreak(line []rune, i int) bool {
	if i >= len(line) {
		return true
	}
	if !IsShellOpAt(line, i) {
		return false
	}
	inQ, _ := QuoteStateAt(line, i)
	return !inQ
}

func wordsInRange(line []rune, start, end int) []Word {
	var words []Word
	i := start
	for i < end {
		for i < end && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i >= end {
			break
		}
		if IsShellOpAt(line, i) {
			break
		}
		wStart := i
		i = scanWordEnd(line, i, end)
		words = append(words, Word{Start: wStart, End: i, Text: string(line[wStart:i])})
	}
	return words
}

func segmentStart(shell []rune, cursor int) int {
	i := cursor - 1
	for i >= 0 {
		if IsShellOpAt(shell, i) {
			return SkipShellOp(shell, i)
		}
		i--
	}
	return 0
}

func WordIndexInSegment(words []Word, tokenStart int) int {
	for i, w := range words {
		if tokenStart >= w.Start && tokenStart < w.End {
			return i
		}
		if tokenStart == w.End {
			return i + 1
		}
	}
	return len(words)
}

func scanWordEnd(shell []rune, i, limit int) int {
	inQuote := false
	var quote rune
	for i < limit {
		ch := shell[i]
		if inQuote {
			if ch == quote && (i == 0 || shell[i-1] != '\\') {
				inQuote = false
			}
			i++
			continue
		}
		if ch == '"' || ch == '\'' {
			inQuote = true
			quote = ch
			i++
			continue
		}
		if ch == ' ' || ch == '\t' {
			if IsEscapedAt(shell, i) {
				i++
				continue
			}
			break
		}
		if IsShellOpAt(shell, i) {
			break
		}
		i++
	}
	return i
}

func IsShellOpAt(shell []rune, i int) bool {
	if i < 0 || i >= len(shell) {
		return false
	}
	switch shell[i] {
	case '|':
		return true
	case ';':
		return true
	case '&':
		return i+1 < len(shell) && shell[i+1] == '&'
	default:
		return false
	}
}

func SkipShellOp(shell []rune, i int) int {
	if i >= len(shell) {
		return i
	}
	switch shell[i] {
	case '|':
		if i+1 < len(shell) && shell[i+1] == '|' {
			return i + 2
		}
		return i + 1
	case ';':
		return i + 1
	case '&':
		if i+1 < len(shell) && shell[i+1] == '&' {
			return i + 2
		}
		return i + 1
	default:
		return i + 1
	}
}

func LooksLikePathToken(token string) bool {
	if token == "" {
		return false
	}
	if token[0] == '"' || token[0] == '\'' {
		return true
	}
	if strings.HasPrefix(token, "~") || strings.Contains(token, "$") {
		return true
	}
	if strings.HasPrefix(token, ".") {
		return true
	}
	if strings.Contains(token, "/") || strings.Contains(token, "\\") {
		return true
	}
	return false
}

func IsGoCommandName(name string) bool {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(strings.ToLower(name), ".exe")
	return name == "go"
}

func InQuoteRange(line []rune, pos int) bool {
	inQ, _ := QuoteStateAt(line, pos+1)
	return inQ
}

func StripOuterQuotes(word string) string {
	word = strings.TrimSpace(word)
	if len(word) < 2 {
		return word
	}
	if (word[0] == '"' && word[len(word)-1] == '"') || (word[0] == '\'' && word[len(word)-1] == '\'') {
		return word[1 : len(word)-1]
	}
	return word
}

func CommandNameFromWord(word string) string {
	w := StripOuterQuotes(word)
	if i := strings.IndexAny(w, "/\\"); i >= 0 {
		w = w[i+1:]
	}
	return strings.TrimSpace(w)
}
