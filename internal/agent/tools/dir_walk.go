package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/gitignore"
)

const (
	defaultTreeMaxDepth   = 6
	defaultTreeMaxEntries = 800
	maxListDirEntries     = 2000
)

type dirBrowseOpts struct {
	IncludeHidden    bool
	RespectGitignore bool
}

type dirEntry struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Size  int64  `json:"size,omitempty"`
}

func pushGitignoreStack(stack *gitignore.Stack, projRoot, absDir string) {
	rel, err := filepath.Rel(projRoot, absDir)
	if err != nil {
		return
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return
	}
	cur := projRoot
	for _, part := range strings.Split(rel, "/") {
		if part == "" || part == "." {
			continue
		}
		cur = filepath.Join(cur, part)
		stack.PushDir(cur)
	}
}

func gitignoreStackAt(projRoot, absDir string, respect bool) *gitignore.Stack {
	if !respect {
		return nil
	}
	stack := gitignore.NewStack(projRoot)
	pushGitignoreStack(stack, projRoot, absDir)
	return stack
}

func shouldListName(name string, includeHidden bool) bool {
	if name == "" {
		return false
	}
	if !includeHidden && strings.HasPrefix(name, ".") {
		return false
	}
	return true
}

func listDirectoryEntries(projRoot, absDir string, opts dirBrowseOpts) ([]dirEntry, error) {
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, err
	}
	stack := gitignoreStackAt(projRoot, absDir, opts.RespectGitignore)
	out := make([]dirEntry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !shouldListName(name, opts.IncludeHidden) {
			continue
		}
		full := filepath.Join(absDir, name)
		if stack != nil && stack.Ignored(full, e.IsDir()) {
			continue
		}
		if e.IsDir() && shouldSkipDir(full, name) {
			continue
		}
		item := dirEntry{Name: name, Type: "file"}
		if e.IsDir() {
			item.Type = "dir"
		} else if info, err := e.Info(); err == nil {
			item.Size = info.Size()
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type == "dir"
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > maxListDirEntries {
		return nil, fmt.Errorf("listDir: too many entries (%d); narrow path or use tree with limits", len(out))
	}
	return out, nil
}

type treeState struct {
	lines     []string
	count     int
	truncated bool
	maxDepth  int
	maxEntries int
	opts      dirBrowseOpts
	projRoot  string
}

func (s *treeState) allowMore() bool {
	if s.count >= s.maxEntries {
		s.truncated = true
		return false
	}
	return true
}

func buildDirectoryTree(projRoot, absDir string, maxDepth, maxEntries int, opts dirBrowseOpts) (string, int, bool, error) {
	rel := relPathOrDot(projRoot, absDir)
	rootLabel := "."
	if rel != "." {
		rootLabel = rel
	}
	st := &treeState{
		maxDepth:   maxDepth,
		maxEntries: maxEntries,
		opts:       opts,
		projRoot:   projRoot,
	}
	st.lines = append(st.lines, rootLabel)
	if err := walkTree(st, absDir, "", 1); err != nil {
		return "", 0, false, err
	}
	return strings.Join(st.lines, "\n"), st.count, st.truncated, nil
}

func walkTree(st *treeState, absDir, prefix string, depth int) error {
	if depth > st.maxDepth {
		st.truncated = true
		return nil
	}
	entries, err := listDirectoryEntries(st.projRoot, absDir, st.opts)
	if err != nil {
		return err
	}
	for i, e := range entries {
		if !st.allowMore() {
			return nil
		}
		st.count++
		isLast := i == len(entries)-1
		branch := "├── "
		nextPrefix := prefix + "│   "
		if isLast {
			branch = "└── "
			nextPrefix = prefix + "    "
		}
		st.lines = append(st.lines, prefix+branch+e.Name)
		if e.Type == "dir" {
			if err := walkTree(st, filepath.Join(absDir, e.Name), nextPrefix, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}
