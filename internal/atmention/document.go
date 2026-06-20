package atmention

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/gitignore"
)

const MaxIncludeDepth = 4
const DefaultMaxFileExpandBytes = 32 * 1024
const replMaxFileExpandBytes = 4 << 20

type DocumentOpts struct {
	ProjRoot     string
	BaseDir      string
	SourcePath   string
	MaxFileBytes int64
	Notify       *Notifier
	Depth        int
	Visited      map[string]struct{}
}

func ExpandDocument(ctx context.Context, text, sourceAbsPath, projRoot string, notify *Notifier) (string, error) {
	return ExpandDocumentWithMax(ctx, text, sourceAbsPath, projRoot, DefaultMaxFileExpandBytes, notify)
}

func ExpandDocumentWithMax(ctx context.Context, text, sourceAbsPath, projRoot string, maxFileBytes int64, notify *Notifier) (string, error) {
	baseDir := strings.TrimSpace(projRoot)
	if sourceAbsPath != "" {
		baseDir = filepath.Dir(filepath.Clean(sourceAbsPath))
	}
	if baseDir == "" {
		baseDir = projRoot
	}
	opts := DocumentOpts{
		ProjRoot:     projRoot,
		BaseDir:      baseDir,
		SourcePath:   sourceAbsPath,
		MaxFileBytes: maxFileBytes,
		Notify:       notify,
		Depth:        0,
		Visited:      make(map[string]struct{}),
	}
	return expandDocumentText(ctx, text, opts)
}

func expandDocumentText(ctx context.Context, text string, opts DocumentOpts) (string, error) {
	tags := FindDocumentTags(text)
	if len(tags) == 0 {
		return text, nil
	}
	var b strings.Builder
	b.WriteString(text)
	for _, tag := range tags {
		block, err := expandDocumentTag(ctx, tag.Path, opts)
		if err != nil {
			continue
		}
		if block != "" {
			b.WriteString(block)
		}
	}
	return b.String(), nil
}

func expandDocumentTag(ctx context.Context, tagPath string, opts DocumentOpts) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	if opts.Depth >= MaxIncludeDepth {
		if opts.Notify != nil {
			opts.Notify.Add(SkipDepth, tagPath, tagPath)
		}
		return "", nil
	}
	abs, err := ResolveDocumentPath(tagPath, opts.BaseDir)
	if err != nil {
		if opts.Notify != nil {
			opts.Notify.Add(SkipMissing, tagPath, tagPath)
		}
		return "", err
	}
	abs = filepath.Clean(abs)
	if opts.Visited != nil {
		if _, ok := opts.Visited[abs]; ok {
			if opts.Notify != nil {
				opts.Notify.Add(SkipCycle, tagPath, abs)
			}
			return "", nil
		}
	}
	st, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			if opts.Notify != nil {
				opts.Notify.Add(SkipMissing, tagPath, abs)
			}
			return "", err
		}
		if opts.Notify != nil {
			opts.Notify.Add(SkipMissing, tagPath, abs)
		}
		return "", err
	}
	label := displayPath(abs, opts.ProjRoot)
	if !underProjectRoot(abs, opts.ProjRoot) && opts.Notify != nil {
		opts.Notify.Add(SkipExternal, tagPath, abs)
	}
	if opts.ProjRoot != "" && isGitignored(abs, st.IsDir(), opts.ProjRoot) {
		if opts.Notify != nil {
			opts.Notify.Add(SkipGitignored, tagPath, abs)
		}
		return "", nil
	}
	if st.IsDir() {
		return fmt.Sprintf("\n\n--- folder %s ---\n%s", label, abs), nil
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		if opts.Notify != nil {
			opts.Notify.Add(SkipMissing, tagPath, abs)
		}
		return "", err
	}
	if len(data) > replMaxFileExpandBytes || isBinary(data) {
		if opts.Notify != nil {
			opts.Notify.Add(SkipBinary, tagPath, abs)
		}
		return "", nil
	}
	if !isAllowedTextFile(filepath.Base(abs), data) {
		if opts.Notify != nil {
			opts.Notify.Add(SkipNotText, tagPath, abs)
		}
		return "", nil
	}
	raw := truncateBytes(data, opts.maxBytes())
	childVisited := copyVisited(opts.Visited)
	if childVisited == nil {
		childVisited = make(map[string]struct{})
	}
	childVisited[abs] = struct{}{}
	childOpts := opts
	childOpts.BaseDir = filepath.Dir(abs)
	childOpts.SourcePath = abs
	childOpts.Depth = opts.Depth + 1
	childOpts.Visited = childVisited
	expanded, err := expandDocumentText(ctx, string(raw), childOpts)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("\n\n--- file %s ---\n%s", label, expanded), nil
}

func (o DocumentOpts) maxBytes() int64 {
	if o.MaxFileBytes <= 0 {
		return DefaultMaxFileExpandBytes
	}
	return o.MaxFileBytes
}

func copyVisited(v map[string]struct{}) map[string]struct{} {
	if len(v) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(v))
	for k := range v {
		out[k] = struct{}{}
	}
	return out
}

func truncateBytes(data []byte, max int64) []byte {
	if max <= 0 || int64(len(data)) <= max {
		return data
	}
	footer := fmt.Sprintf("\n\n[truncated: %d bytes omitted — edit file to reduce]", int64(len(data))-max)
	cut := max - int64(len(footer))
	if cut < 0 {
		cut = 0
	}
	return append(data[:cut], footer...)
}

func isGitignored(absPath string, isDir bool, projRoot string) bool {
	root, err := filepath.Abs(projRoot)
	if err != nil {
		return false
	}
	absPath = filepath.Clean(absPath)
	if !underProjectRoot(absPath, root) {
		return false
	}
	stack := gitignore.NewStack(root)
	rel, err := filepath.Rel(root, absPath)
	if err != nil || rel == "." {
		return stack.Ignored(absPath, isDir)
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	cur := root
	for i, part := range parts {
		if part == "" {
			continue
		}
		cur = filepath.Join(cur, part)
		last := i == len(parts)-1
		if last {
			return stack.Ignored(absPath, isDir)
		}
		if st, statErr := os.Stat(cur); statErr == nil && st.IsDir() {
			stack.PushDir(cur)
		}
	}
	return stack.Ignored(absPath, isDir)
}
