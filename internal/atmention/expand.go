package atmention

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func ExpandLine(ctx context.Context, visible, projRoot string, index []Entry) (apiContent string, err error) {
	if strings.TrimSpace(visible) == "" {
		return "", nil
	}
	bounds := TagRuneBounds([]rune(visible))
	if len(bounds) == 0 {
		return visible, nil
	}
	root, err := filepath.Abs(projRoot)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(visible)
	for _, tagBounds := range bounds {
		tag := string([]rune(visible)[tagBounds.Start+1 : tagBounds.End])
		entry, ok := ResolveTag(tag, index)
		if !ok {
			b.WriteString(fmt.Sprintf("\n\n[atmention: could not resolve @%s]", tag))
			continue
		}
		abs := filepath.Join(root, filepath.FromSlash(entry.RelPath))
		if entry.IsDir {
			absPath, absErr := filepath.Abs(abs)
			if absErr != nil {
				b.WriteString(fmt.Sprintf("\n\n[atmention: folder @%s: %v]", tag, absErr))
				continue
			}
			b.WriteString(fmt.Sprintf("\n\n--- folder %s ---\n%s", entry.RelPath, absPath))
			continue
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			b.WriteString(fmt.Sprintf("\n\n[atmention: file @%s: %v]", tag, err))
			continue
		}
		if isBinary(data) {
			b.WriteString(fmt.Sprintf("\n\n[atmention: file @%s: binary file cannot be attached]", tag))
			continue
		}
		b.WriteString(fmt.Sprintf("\n\n--- file %s ---\n%s", entry.RelPath, string(data)))
	}
	return b.String(), nil
}

func isBinary(data []byte) bool {
	n := len(data)
	if n > 8192 {
		n = 8192
	}
	for i := 0; i < n; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}


type TerminalClip struct {
	End   int
	Start int
	Tag   string
	Text  string
}

var terminalTagRE = regexp.MustCompile(`\[terminal-L([0-9]+)-L([0-9]+)\]`)

func ExpandTerminalClips(visible, apiContent string, clips map[string]string) string {
	if clips == nil || !strings.Contains(visible, "[terminal-L") {
		return apiContent
	}
	base := apiContent
	if strings.TrimSpace(base) == "" {
		base = visible
	}
	var b strings.Builder
	b.WriteString(base)
	seen := map[string]bool{}
	for _, match := range terminalTagRE.FindAllString(visible, -1) {
		if seen[match] {
			continue
		}
		seen[match] = true
		body, ok := clips[match]
		if !ok || strings.TrimSpace(body) == "" {
			b.WriteString("\n\n[terminal: missing selection " + match + "]")
			continue
		}
		b.WriteString("\n\n--- terminal " + match + " ---\n")
		b.WriteString(body)
	}
	return b.String()
}

func MergeTerminalClips(dst map[string]string, incoming []TerminalClip) map[string]string {
	if len(incoming) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]string, len(incoming))
	}
	for _, clip := range incoming {
		tag := strings.TrimSpace(clip.Tag)
		if tag == "" {
			tag = fmt.Sprintf("[terminal-L%d-L%d]", clip.Start, clip.End)
		}
		dst[tag] = clip.Text
	}
	return dst
}
