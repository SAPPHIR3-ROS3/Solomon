package filechanges

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Snapshots are private and outside the project: no index or worktree mutations.
type Snapshot struct {
	root, dir string
	files     map[string]bool
	locations map[string]string
}

func (s *Snapshot) Close()                   { _ = os.RemoveAll(s.dir) }
func Capture(root string) (*Snapshot, error) { return CapturePaths(root, nil) }

// CapturePaths snapshots explicit tool targets, including ignored files and
// files outside the project. No file is ever copied through a ../ temp path.
func CapturePaths(root string, targets []string) (*Snapshot, error) {
	dir, err := os.MkdirTemp("", "solomon-changes-")
	if err != nil {
		return nil, err
	}
	s := &Snapshot{root: root, dir: dir, files: map[string]bool{}, locations: map[string]string{}}
	paths := map[string]bool{}
	if len(targets) == 0 {
		names, err := projectChangePaths(root)
		if err != nil {
			s.Close()
			return nil, err
		}
		for _, name := range names {
			paths[filepath.Join(root, name)] = true
		}
	} else {
		for _, target := range targets {
			if !filepath.IsAbs(target) {
				target = filepath.Join(root, target)
			}
			info, err := os.Stat(target)
			if err == nil && info.IsDir() {
				names, err := projectChangePaths(target)
				if err != nil {
					s.Close()
					return nil, err
				}
				for _, name := range names {
					paths[filepath.Join(target, name)] = true
				}
			} else {
				paths[filepath.Clean(target)] = true
			}
		}
	}
	for source := range paths {
		info, err := os.Stat(source)
		if os.IsNotExist(err) || errors.Is(err, os.ErrPermission) {
			continue
		}
		if err != nil {
			s.Close()
			return nil, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		abs, err := filepath.Abs(source)
		if err != nil {
			s.Close()
			return nil, err
		}
		name, err := filepath.Rel(root, abs)
		if err != nil || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			name = abs
		}
		digest := sha256.Sum256([]byte(abs))
		target := filepath.Join(dir, hex.EncodeToString(digest[:]))
		if err := copyChangeFile(source, target); err != nil {
			if errors.Is(err, os.ErrPermission) || os.IsNotExist(err) {
				continue
			}
			s.Close()
			return nil, err
		}
		s.files[name] = true
		s.locations[name] = target
	}
	return s, nil
}
func copyChangeFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func projectChangePaths(root string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", ".")
	if output, err := cmd.Output(); err == nil {
		var names []string
		for _, name := range strings.Split(string(output), "\x00") {
			if name != "" {
				names = append(names, filepath.FromSlash(name))
			}
		}
		return names, nil
	}
	var names []string
	home, _ := os.UserHomeDir()
	shallow := filepath.Clean(root) == filepath.Clean(home) || filepath.Clean(root) == string(filepath.Separator)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrPermission) {
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return err
		}
		if d.IsDir() {
			if (shallow && path != root) || skipSnapshotDirectory(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			names = append(names, rel)
		}
		return nil
	})
	return names, err
}

type Change struct {
	AddedLines, RemovedLines  map[string]int
	Name                      string
	Added, Removed            int
	Binary                    bool
	Lines                     map[string]int
	BeforeExists, AfterExists bool
}

func Compare(before, after *Snapshot) ([]Change, error) {
	names := map[string]bool{}
	for name := range before.files {
		names[name] = true
	}
	for name := range after.files {
		names[name] = true
	}
	var changes []Change
	for name := range names {
		oldPath, newPath := before.locations[name], after.locations[name]
		old, _ := os.ReadFile(oldPath)
		next, _ := os.ReadFile(newPath)
		if before.files[name] == after.files[name] && bytes.Equal(old, next) {
			continue
		}
		if !before.files[name] {
			oldPath = os.DevNull
		}
		if !after.files[name] {
			newPath = os.DevNull
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		output, err := exec.CommandContext(ctx, "git", "diff", "--no-index", "--no-ext-diff", "--no-textconv", "--unified=0", "--", oldPath, newPath).Output()
		cancel()
		if err != nil {
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
				return nil, err
			}
		}
		c := ParsePatch(filepath.ToSlash(name), output)
		c.BeforeExists = before.files[name]
		c.AfterExists = after.files[name]
		changes = append(changes, c)
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Name < changes[j].Name })
	return changes, nil
}

// Commit describes the paths committed by the current chat's own tool.
type Commit struct {
	Name  string
	Lines map[string]int
	Clean bool
}

func ParsePatch(name string, output []byte) Change {
	c := Change{Name: name, Lines: map[string]int{}, AddedLines: map[string]int{}, RemovedLines: map[string]int{}}
	inHunk := false
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "Binary files ") {
			c.Binary = true
		}
		if strings.HasPrefix(line, "@@") {
			inHunk = true
			continue
		}
		if !inHunk || len(line) == 0 {
			continue
		}
		switch line[0] {
		case '+':
			c.Added++
			c.AddedLines[line[1:]]++
			c.Lines[line[1:]]++
		case '-':
			c.Removed++
			c.RemovedLines[line[1:]]++
			c.Lines[line[1:]]--
		}
	}
	return c
}

func skipSnapshotDirectory(name string) bool {
	switch name {
	case ".git", ".solomon", ".cache", ".local", "node_modules", ".venv", "venv":
		return true
	}
	return false
}
