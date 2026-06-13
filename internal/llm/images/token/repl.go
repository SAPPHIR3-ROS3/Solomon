package token

import (
	"regexp"
	"strconv"
	"strings"
)

type RuneBounds struct {
	Start int
	End   int
}

func collapseTokenToVisible(tag string) string {
	if sm := storedTokenRE.FindStringSubmatch(tag); len(sm) >= 2 {
		seq, err := strconv.Atoi(sm[1])
		if err == nil {
			return VisibleTag(seq)
		}
	}
	if sm := legacyHexRE.FindStringSubmatch(tag); len(sm) >= 2 {
		seq, err := strconv.Atoi(sm[1])
		if err == nil {
			return VisibleTag(seq)
		}
	}
	return tag
}

func wireTokenRuneBounds(line []rune) []RuneBounds {
	lineStr := string(line)
	var out []RuneBounds
	for _, re := range []*regexp.Regexp{storedTokenRE, legacyHexRE} {
		for _, loc := range re.FindAllStringSubmatchIndex(lineStr, -1) {
			b := RuneBounds{
				Start: runeIndexAtByte(lineStr, loc[0]),
				End:   runeIndexAtByte(lineStr, loc[1]),
			}
			overlap := false
			for _, x := range out {
				if boundsOverlap(b, x) {
					overlap = true
					break
				}
			}
			if !overlap {
				out = append(out, b)
			}
		}
	}
	sortBounds(out)
	return out
}

func sortBounds(bounds []RuneBounds) {
	for i := 0; i < len(bounds); i++ {
		for j := i + 1; j < len(bounds); j++ {
			if bounds[j].Start < bounds[i].Start {
				bounds[i], bounds[j] = bounds[j], bounds[i]
			}
		}
	}
}

func NormalizeREPLBuffer(line []rune, pos int) ([]rune, int) {
	bounds := wireTokenRuneBounds(line)
	if len(bounds) == 0 {
		return line, pos
	}
	out := make([]rune, 0, len(line))
	last := 0
	newPos := pos
	for _, b := range bounds {
		out = append(out, line[last:b.Start]...)
		tag := string(line[b.Start:b.End])
		visible := collapseTokenToVisible(tag)
		if visible == tag {
			out = append(out, line[b.Start:b.End]...)
			last = b.End
			continue
		}
		vis := []rune(visible)
		ins := len(out)
		oldLen := b.End - b.Start
		visLen := len(vis)
		switch {
		case pos <= b.Start:
		case pos > b.End:
			newPos -= oldLen - visLen
		default:
			newPos = ins + visLen
		}
		out = append(out, vis...)
		last = b.End
	}
	out = append(out, line[last:]...)
	if newPos < 0 {
		newPos = 0
	}
	if newPos > len(out) {
		newPos = len(out)
	}
	return out, newPos
}

func boundsOverlap(a, b RuneBounds) bool {
	return a.Start < b.End && b.Start < a.End
}

func ImgTagRuneBounds(line []rune) []RuneBounds {
	lineStr := string(line)
	var out []RuneBounds
	for _, loc := range completeTokenRE.FindAllStringSubmatchIndex(lineStr, -1) {
		out = append(out, RuneBounds{
			Start: runeIndexAtByte(lineStr, loc[0]),
			End:   runeIndexAtByte(lineStr, loc[1]),
		})
	}
	for _, loc := range legacyBareRE.FindAllStringSubmatchIndex(lineStr, -1) {
		b := RuneBounds{
			Start: runeIndexAtByte(lineStr, loc[0]),
			End:   runeIndexAtByte(lineStr, loc[1]),
		}
		overlap := false
		for _, x := range out {
			if boundsOverlap(b, x) {
				overlap = true
				break
			}
		}
		if !overlap {
			out = append(out, b)
		}
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

func MaskPUAPayloadForDisplay(line []rune) []rune {
	bounds := ImgTagRuneBounds(line)
	if len(bounds) == 0 {
		return line
	}
	out := make([]rune, len(line))
	copy(out, line)
	for _, b := range bounds {
		for i := b.Start; i < b.End; i++ {
			if line[i] >= puaBase && line[i] <= puaEnd {
				out[i] = PayloadSep
			}
		}
	}
	return out
}

func deleteRuneRange(line []rune, start, end int) ([]rune, int) {
	newLine := append(append([]rune(nil), line[:start]...), line[end:]...)
	return newLine, start
}

func imgLiteralRuneBounds(line []rune) []RuneBounds {
	lineStr := string(line)
	locs := userLiteralRE.FindAllStringSubmatchIndex(lineStr, -1)
	if len(locs) == 0 {
		return nil
	}
	out := make([]RuneBounds, 0, len(locs))
	for _, loc := range locs {
		out = append(out, RuneBounds{
			Start: runeIndexAtByte(lineStr, loc[0]),
			End:   runeIndexAtByte(lineStr, loc[1]),
		})
	}
	return out
}

func isImgTokenRune(r rune) bool {
	if r == PayloadSep || r == '[' || r == ']' {
		return true
	}
	if r >= puaBase && r <= puaEnd {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	switch r {
	case 'i', 'm', 'g', '-':
		return true
	}
	return false
}

func imgTagSpanContaining(line []rune, pos int) (start, end int, ok bool) {
	if pos <= 0 || pos > len(line) {
		return 0, 0, false
	}
	if !isImgTokenRune(line[pos-1]) {
		return 0, 0, false
	}
	start = pos - 1
	for start > 0 && isImgTokenRune(line[start-1]) {
		start--
	}
	end = pos
	for end < len(line) && isImgTokenRune(line[end]) {
		end++
	}
	if end < len(line) && line[end] == ']' {
		end++
	}
	if !strings.Contains(string(line[start:end]), "[img-") {
		return 0, 0, false
	}
	return start, end, true
}

func BackspaceImgFragment(line []rune, pos int) ([]rune, int, bool) {
	if start, end, ok := imgTagSpanContaining(line, pos); ok {
		newLine, newPos := deleteRuneRange(line, start, end)
		return newLine, newPos, true
	}
	return nil, 0, false
}

func DeleteImgTagAt(line []rune, pos int) ([]rune, int, bool) {
	for _, b := range ImgTagRuneBounds(line) {
		if pos > b.Start && pos <= b.End {
			newLine, newPos := deleteRuneRange(line, b.Start, b.End)
			return newLine, newPos, true
		}
	}
	for _, b := range imgLiteralRuneBounds(line) {
		if pos > b.Start && pos <= b.End {
			newLine, newPos := deleteRuneRange(line, b.Start, b.End)
			return newLine, newPos, true
		}
	}
	return nil, 0, false
}

func DeleteImgTagForward(line []rune, pos int) ([]rune, int, bool) {
	for _, b := range ImgTagRuneBounds(line) {
		if pos >= b.Start && pos < b.End {
			newLine, newPos := deleteRuneRange(line, b.Start, b.End)
			return newLine, newPos, true
		}
	}
	for _, b := range imgLiteralRuneBounds(line) {
		if pos >= b.Start && pos < b.End {
			newLine, newPos := deleteRuneRange(line, b.Start, b.End)
			return newLine, newPos, true
		}
	}
	return nil, 0, false
}
