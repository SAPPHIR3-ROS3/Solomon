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
	lines := strings.Split(s, "\n")
	var out []DocumentTag
	offset := 0
	inFence := false
	fenceMarker := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inFence {
			if m, ok := fenceOpen(trimmed); ok {
				inFence = true
				fenceMarker = m
				offset += len(line) + 1
				continue
			}
			out = append(out, scanPlainLine(line, offset)...)
		} else if fenceClose(trimmed, fenceMarker) {
			inFence = false
			fenceMarker = ""
		}
		offset += len(line) + 1
	}
	return out
}

func fenceOpen(trimmed string) (marker string, ok bool) {
	for _, m := range []string{"```", "~~~"} {
		if trimmed == m || strings.HasPrefix(trimmed, m+" ") {
			return m, true
		}
	}
	return "", false
}

func fenceClose(trimmed, marker string) bool {
	return trimmed == marker || strings.HasPrefix(trimmed, marker+" ")
}

func scanPlainLine(line string, lineOffset int) []DocumentTag {
	var out []DocumentTag
	inInline := false
	i := 0
	for i < len(line) {
		if line[i] == '`' {
			inInline = !inInline
			i++
			continue
		}
		if !inInline && line[i] == '@' {
			rest := line[i:]
			loc := documentTagRE.FindStringSubmatchIndex(rest)
			if loc == nil {
				i++
				continue
			}
			path := rest[loc[2]:loc[3]]
			out = append(out, DocumentTag{
				Start: lineOffset + i,
				End:   lineOffset + i + loc[1],
				Path:  path,
			})
			i += loc[1]
			continue
		}
		i++
	}
	return out
}
