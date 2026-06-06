package atmention

import (
	"regexp"
)

var atTagRE = regexp.MustCompile(`@([^\s@]+)`)

type RuneBounds struct {
	Start int
	End   int
}

func TagRuneBounds(line []rune) []RuneBounds {
	lineStr := string(line)
	var out []RuneBounds
	for _, loc := range atTagRE.FindAllStringSubmatchIndex(lineStr, -1) {
		out = append(out, RuneBounds{
			Start: runeIndexAtByte(lineStr, loc[0]),
			End:   runeIndexAtByte(lineStr, loc[1]),
		})
	}
	return out
}

func runeIndexAtByte(s string, b int) int {
	if b <= 0 {
		return 0
	}
	if b >= len(s) {
		return len([]rune(s))
	}
	return len([]rune(s[:b]))
}

func JumpLeftOverTag(line []rune, pos int) int {
	if pos <= 0 {
		return -1
	}
	for _, b := range TagRuneBounds(line) {
		if pos > b.Start && pos <= b.End {
			return b.Start
		}
	}
	return -1
}

func JumpRightOverTag(line []rune, pos int) int {
	if pos >= len(line) {
		return -1
	}
	for _, b := range TagRuneBounds(line) {
		if pos >= b.Start && pos < b.End {
			return b.End
		}
	}
	return -1
}

func DeleteTagAt(line []rune, pos int) ([]rune, int, bool) {
	for _, b := range TagRuneBounds(line) {
		if pos > b.Start && pos <= b.End {
			return deleteRuneRange(line, b.Start, b.End)
		}
	}
	return nil, 0, false
}

func DeleteTagForward(line []rune, pos int) ([]rune, int, bool) {
	for _, b := range TagRuneBounds(line) {
		if pos >= b.Start && pos < b.End {
			return deleteRuneRange(line, b.Start, b.End)
		}
	}
	return nil, 0, false
}

func deleteRuneRange(line []rune, start, end int) ([]rune, int, bool) {
	newLine := append(append([]rune(nil), line[:start]...), line[end:]...)
	return newLine, start, true
}

type AtContext struct {
	Active       bool
	AtStart      int
	QueryStart   int
	Query        string
	ReplaceStart int
}

func AtContextAt(line []rune, col int) AtContext {
	if col < 0 {
		col = 0
	}
	if col > len(line) {
		col = len(line)
	}
	at := -1
	for i := col - 1; i >= 0; i-- {
		r := line[i]
		if r == '@' {
			at = i
			break
		}
		if r == ' ' || r == '\t' || r == '\n' {
			break
		}
	}
	if at < 0 {
		return AtContext{}
	}
	queryRunes := line[at+1 : col]
	for _, r := range queryRunes {
		if r == ' ' || r == '\t' || r == '\n' {
			return AtContext{}
		}
	}
	return AtContext{
		Active:       true,
		AtStart:      at,
		QueryStart:   at + 1,
		Query:        string(queryRunes),
		ReplaceStart: at,
	}
}
