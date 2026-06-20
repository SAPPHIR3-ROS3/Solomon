package atmention

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
		if len(data) > replMaxFileExpandBytes || isBinary(data) {
			b.WriteString(fmt.Sprintf("\n\n[atmention: file @%s: binary or too large to attach]", tag))
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
