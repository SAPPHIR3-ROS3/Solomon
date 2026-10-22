package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/filechanges"
	"github.com/gofrs/flock"
)

// All Solomon shell/edit calls share the project lock, including calls from
// chats where rendering is disabled. A snapshot can therefore only contain
// changes made by the tool that owns the lock, not another Solomon chat.
func WithFileChangeTracking(ctx context.Context, env *Env, run func() (any, error)) (any, error) {
	if env == nil || env.ProjRoot == "" {
		return run()
	}
	return WithFileChangeTrackingPaths(ctx, env, nil, run)
}

func WithFileChangeTrackingPaths(ctx context.Context, env *Env, paths []string, run func() (any, error)) (any, error) {
	if env == nil || env.ProjRoot == "" {
		return run()
	}
	var err error
	// Absolute tool targets may overlap even when the chats have different
	// project roots. Serialize Solomon writers across roots, too.
	lock := flock.New(filepath.Join(os.TempDir(), fmt.Sprintf("solomon-file-changes-%x.lock", sha256.Sum256([]byte(os.Getenv("USER"))))))
	ok, err := lock.TryLockContext(ctx, 25*time.Millisecond)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("project file change lock unavailable")
	}
	defer lock.Unlock()
	if env.RecordFileChanges == nil {
		return run()
	}
	var before *filechanges.Snapshot
	if env.Cfg == nil || env.Cfg.FileChangesEnabled() {
		before, err = filechanges.CapturePaths(env.ProjRoot, paths)
		if err != nil && env.FileChangesError != nil {
			env.FileChangesError(err)
		}
		if before != nil {
			defer before.Close()
		}
	}
	headBefore := changeHead(ctx, env.ProjRoot)
	result, runErr := run()
	var changes []filechanges.Change
	if before != nil {
		after, captureErr := filechanges.CapturePaths(env.ProjRoot, paths)
		if captureErr != nil {
			if env.FileChangesError != nil {
				env.FileChangesError(captureErr)
			}
		} else {
			defer after.Close()
			changes, err = filechanges.Compare(before, after)
			if err != nil && env.FileChangesError != nil {
				env.FileChangesError(err)
			}
		}
	}
	// Finishing bookkeeping must also run after a cancelled shell command.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	var committed []filechanges.Commit
	headAfter := changeHead(ctx, env.ProjRoot)
	if headAfter != "" && headAfter != headBefore && isToolCommit(ctx, env.ProjRoot, headAfter) {
		args := []string{"-C", env.ProjRoot, "diff-tree", "--no-commit-id", "--relative", "--name-only", "-r", "-z", "--root", headAfter}
		if headBefore != "" {
			args = []string{"-C", env.ProjRoot, "diff", "--relative", "--name-only", "-z", headBefore, headAfter, "--", "."}
		}
		if out, err := exec.CommandContext(ctx, "git", args...).Output(); err == nil {
			for _, name := range strings.Split(string(out), "\x00") {
				if name != "" {
					committed = append(committed, committedChange(ctx, env.ProjRoot, headBefore, headAfter, name))
				}
			}
		}
	}
	env.RecordFileChanges(changes, committed)
	return result, runErr
}
func changeHead(ctx context.Context, root string) string {
	out, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--verify", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func committedChange(ctx context.Context, root, before, after, name string) filechanges.Commit {
	args := []string{"-C", root, "diff", "--no-ext-diff", "--no-textconv", "--unified=0", before, after, "--", name}
	if before == "" {
		args = []string{"-C", root, "show", "--format=", "--root", "--no-ext-diff", "--no-textconv", "--unified=0", after, "--", name}
	}
	patch, _ := exec.CommandContext(ctx, "git", args...).Output()
	prefix, _ := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--show-prefix").Output()
	committed, commitErr := exec.CommandContext(ctx, "git", "-C", root, "show", after+":"+strings.TrimSpace(string(prefix))+name).Output()
	current, currentErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	clean := commitErr == nil && currentErr == nil && bytes.Equal(committed, current)
	if commitErr != nil && os.IsNotExist(currentErr) {
		clean = true
	}
	return filechanges.Commit{Name: name, Lines: filechanges.ParsePatch(name, patch).Lines, Clean: clean}
}

// A checkout/reset also changes HEAD, but is not a commit made by this chat.
func isToolCommit(ctx context.Context, root, head string) bool {
	out, err := exec.CommandContext(ctx, "git", "-C", root, "reflog", "show", "-1", "--format=%H%x00%gs", "HEAD").Output()
	if err != nil {
		return false
	}
	oid, action, ok := strings.Cut(strings.TrimSpace(string(out)), "\x00")
	return ok && oid == head && (strings.HasPrefix(action, "commit:") || strings.HasPrefix(action, "commit ("))
}
