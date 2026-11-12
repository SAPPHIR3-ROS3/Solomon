package atmention

import (
	"regexp"
	"strings"
)

var documentTagRE = regexp.MustCompile(`@([^\s@]+)`)

type DocumentTag struct {
	Start int
	End   int
	Path  string
}

func FindDocumentTags(s string) []DocumentTag {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []DocumentTag
	offset := 0
	plainStart := 0
	fenceMarker := ""
	for _, line := range strings.SplitAfter(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if fenceMarker == "" {
			if marker, ok := fenceOpen(trimmed); ok {
				out = append(out, scanPlainText(s[plainStart:offset], plainStart)...)
				fenceMarker = marker
			}
		} else if fenceClose(trimmed, fenceMarker) {
			fenceMarker = ""
			plainStart = offset + len(line)
		}
		offset += len(line)
	}
	if fenceMarker == "" {
		out = append(out, scanPlainText(s[plainStart:], plainStart)...)
	}
	return out
}

func fenceOpen(trimmed string) (marker string, ok bool) {
	if len(trimmed) < 3 || (trimmed[0] != '`' && trimmed[0] != '~') {
		return "", false
	}
	end := delimiterEnd(trimmed, 0)
	if end < 3 || (trimmed[0] == '`' && strings.Contains(trimmed[end:], "`")) {
		return "", false
	}
	return trimmed[:end], true
}

func fenceClose(trimmed, marker string) bool {
	if trimmed == "" || trimmed[0] != marker[0] {
		return false
	}
	end := delimiterEnd(trimmed, 0)
	return end >= len(marker) && strings.TrimSpace(trimmed[end:]) == ""
}

func delimiterEnd(text string, start int) int {
	end := start + 1
	for end < len(text) && text[end] == text[start] {
		end++
	}
	return end
}

func inlineCodeEnd(text string, start, width int) int {
	for i := start; i < len(text); {
		if text[i] != '`' {
			i++
			continue
		}
		end := delimiterEnd(text, i)
		if end-i == width {
			return end
		}
		i = end
	}
	return -1
}

func scanPlainText(text string, textOffset int) []DocumentTag {
	var out []DocumentTag
	i := 0
	for i < len(text) {
		if text[i] == '`' {
			end := delimiterEnd(text, i)
			if close := inlineCodeEnd(text, end, end-i); close >= 0 {
				i = close
			} else {
				i = end
			}
			continue
		}
		if text[i] == '@' {
			rest := text[i:]
			loc := documentTagRE.FindStringSubmatchIndex(rest)
			if loc == nil {
				i++
				continue
			}
			path := rest[loc[2]:loc[3]]
			out = append(out, DocumentTag{
				Start: textOffset + i,
				End:   textOffset + i + loc[1],
				Path:  path,
			})
			i += loc[1]
			continue
		}
		i++
	}
	return out
}
