package atmention

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var errTreeLimit = errors.New("tree limit")

const maxTreeEntries = 500
const maxFileExpandBytes = 4 << 20

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
			tree, err := dirTree(ctx, abs, entry.RelPath, maxTreeEntries)
			if err != nil {
				b.WriteString(fmt.Sprintf("\n\n[atmention: folder @%s: %v]", tag, err))
				continue
			}
			b.WriteString(fmt.Sprintf("\n\n--- folder %s ---\n%s", entry.RelPath, tree))
			continue
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			b.WriteString(fmt.Sprintf("\n\n[atmention: file @%s: %v]", tag, err))
			continue
		}
		if len(data) > maxFileExpandBytes || isBinary(data) {
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

func dirTree(ctx context.Context, absDir, relRoot string, limit int) (string, error) {
	var lines []string
	count := 0
	err := filepath.WalkDir(absDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if path == absDir {
			return nil
		}
		rel, err := filepath.Rel(absDir, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			lines = append(lines, rel+"/")
		} else {
			lines = append(lines, rel)
		}
		count++
		if count >= limit {
			return errTreeLimit
		}
		return nil
	})
	if err != nil && err != errTreeLimit {
		return "", err
	}
	sortSliceEntries(lines)
	out := strings.Join(lines, "\n")
	if err == errTreeLimit {
		out += fmt.Sprintf("\n... (truncated at %d entries)", limit)
	}
	return out, nil
}

func sortSliceEntries(lines []string) {
	sort.Strings(lines)
}
