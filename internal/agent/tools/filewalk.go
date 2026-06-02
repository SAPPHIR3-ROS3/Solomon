package tools

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/gitignore"
)

const (
	maxFindFileBytes = 4 << 20
	binaryProbeSize  = 8192
)

type fileCandidate struct {
	AbsPath string
	RelPath string
	IsDir   bool
}

type fileWalkOpts struct {
	Root             string
	RespectGitignore bool
	MaxFileBytes     int64
	SkipBinary       bool
}

func parallelFileWalk(ctx context.Context, opts fileWalkOpts) (<-chan fileCandidate, error) {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}
	if opts.MaxFileBytes <= 0 {
		opts.MaxFileBytes = maxFindFileBytes
	}
	out := make(chan fileCandidate, 256)
	go func() {
		defer close(out)
		var stack *gitignore.Stack
		if opts.RespectGitignore {
			stack = gitignore.NewStack(root)
		}
		var dirStack []string
		if stack != nil {
			dirStack = []string{root}
		}
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if stack != nil && path != root {
				parent := filepath.Dir(path)
				for len(dirStack) > 0 && dirStack[len(dirStack)-1] != parent {
					stack.PopDir()
					dirStack = dirStack[:len(dirStack)-1]
				}
			}
			if d.IsDir() {
				if path != root {
					if shouldSkipDir(path, d.Name()) {
						return filepath.SkipDir
					}
					if stack != nil {
						stack.PushDir(path)
						dirStack = append(dirStack, path)
					}
					if stack != nil && stack.Ignored(path, true) {
						return filepath.SkipDir
					}
				}
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			if stack != nil && stack.Ignored(path, false) {
				return nil
			}
			if stack != nil && strings.HasPrefix(d.Name(), ".") {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if info.Size() > opts.MaxFileBytes {
				return nil
			}
			if opts.SkipBinary && isBinaryFile(path) {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case out <- fileCandidate{AbsPath: path, RelPath: rel, IsDir: false}:
				return nil
			}
		})
	}()
	return out, nil
}

func shouldSkipDir(path, name string) bool {
	switch name {
	case ".git", ".hg", ".svn":
		return true
	default:
		return false
	}
}

func isBinaryFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return true
	}
	defer f.Close()
	buf := make([]byte, binaryProbeSize)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return true
	}
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}
	return false
}
