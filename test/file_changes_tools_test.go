package test

import (
	"context"
	"encoding/json"
	. "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/filechanges"
)

func TestFileChangesIsolateConcurrentChats(t *testing.T) {
	root := t.TempDir()
	if err := exec.Command("git", "init", "--quiet", root).Run(); err != nil {
		t.Skip(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	second := make(chan struct{})
	var a, b []filechanges.Change
	envA := &Env{ProjRoot: root, RecordFileChanges: func(c []filechanges.Change, _ []filechanges.Commit) { a = append(a, c...) }}
	envB := &Env{ProjRoot: root, RecordFileChanges: func(c []filechanges.Change, _ []filechanges.Commit) { b = append(b, c...) }}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := WithFileChangeTracking(context.Background(), envA, func() (any, error) {
			close(entered)
			<-release
			return nil, os.WriteFile(filepath.Join(root, "a.txt"), []byte("chat a\n"), 0600)
		})
		if err != nil {
			t.Error(err)
		}
	}()
	<-entered
	go func() {
		defer wg.Done()
		_, err := WithFileChangeTracking(context.Background(), envB, func() (any, error) {
			close(second)
			return nil, os.WriteFile(filepath.Join(root, "b.txt"), []byte("chat b\n"), 0600)
		})
		if err != nil {
			t.Error(err)
		}
	}()
	select {
	case <-second:
		t.Error("second chat entered before first released project lock")
	case <-time.After(60 * time.Millisecond):
	}
	close(release)
	wg.Wait()
	if len(a) != 1 || a[0].Name != "a.txt" || len(b) != 1 || b[0].Name != "b.txt" {
		t.Fatalf("bleeding: a=%+v b=%+v", a, b)
	}
}
func TestFileChangesCommitIsOwnedByCallingChat(t *testing.T) {
	root := t.TempDir()
	if err := exec.Command("git", "init", "--quiet", root).Run(); err != nil {
		t.Skip(err)
	}
	var aCommits, bCommits []filechanges.Commit
	a := &Env{ProjRoot: root, RecordFileChanges: func(_ []filechanges.Change, c []filechanges.Commit) { aCommits = append(aCommits, c...) }}
	b := &Env{ProjRoot: root, RecordFileChanges: func(_ []filechanges.Change, c []filechanges.Commit) { bCommits = append(bCommits, c...) }}
	run := func(env *Env, fn func() (any, error)) {
		t.Helper()
		if _, err := WithFileChangeTracking(context.Background(), env, fn); err != nil {
			t.Fatal(err)
		}
	}
	run(a, func() (any, error) { return nil, os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0600) })
	run(b, func() (any, error) {
		if err := exec.Command("git", "-C", root, "add", "a.txt").Run(); err != nil {
			return nil, err
		}
		return nil, exec.Command("git", "-C", root, "-c", "user.name=Test", "-c", "user.email=test@example.test", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "fixture").Run()
	})
	run(a, func() (any, error) { return nil, nil })
	if len(aCommits) != 0 || len(bCommits) != 1 || bCommits[0].Name != "a.txt" {
		t.Fatalf("commit bleeding: a=%v b=%v", aCommits, bCommits)
	}
}

func TestFileChangesDesktopShellFromHomeProject(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("desktop shell tracking uses a POSIX shell")
	}
	logging.LogInit(logging.INFO_LOG_LEVEL)
	home := t.TempDir()
	t.Setenv("HOME", home)
	desktop := filepath.Join(home, "Desktop")
	if err := os.MkdirAll(desktop, 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(desktop, "test.txt")
	var changes []filechanges.Change
	env := &Env{ProjRoot: home, AllowDeferredTools: true, RecordFileChanges: func(c []filechanges.Change, _ []filechanges.Commit) { changes = append(changes, c...) }}
	create := `printf '%s\n' 'own line' > "$HOME/Desktop/test.txt"`
	paths := ShellFileChangePaths(home, create)
	found := false
	for _, path := range paths {
		if path == target {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing literal Desktop target: %v", paths)
	}
	raw, _ := json.Marshal(map[string]any{"command": create, "intent": "create isolated desktop fixture"})
	if _, err := Exec(context.Background(), env, "agent", tooling.Invocation{Name: "shell", Args: raw}); err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Name != "Desktop/test.txt" || changes[0].Added != 1 {
		t.Fatalf("create missing: %+v", changes)
	}
	changes = nil
	raw, _ = json.Marshal(map[string]any{"command": `rm -- "$HOME/Desktop/test.txt"`, "intent": "remove isolated desktop fixture"})
	if _, err := Exec(context.Background(), env, "agent", tooling.Invocation{Name: "shell", Args: raw}); err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Removed != 1 {
		t.Fatalf("delete missing: %+v", changes)
	}
}
func TestFileChangesExplicitEditOutsideProject(t *testing.T) {
	logging.LogInit(logging.INFO_LOG_LEVEL)
	root, outside := t.TempDir(), t.TempDir()
	target := filepath.Join(outside, "external.txt")
	var changes []filechanges.Change
	env := &Env{ProjRoot: root, AllowDeferredTools: true, RecordFileChanges: func(c []filechanges.Change, _ []filechanges.Commit) { changes = append(changes, c...) }}
	raw, _ := json.Marshal(map[string]any{"path": target, "newString": "own line\n", "intent": "create isolated external fixture"})
	if _, err := Exec(context.Background(), env, "agent", tooling.Invocation{Name: "editFile", Args: raw}); err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Name != filepath.ToSlash(target) {
		t.Fatalf("external edit missing: %+v", changes)
	}
}

func TestFileChangesXDGDesktopVariableCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG desktop commands use a POSIX shell")
	}
	logging.LogInit(logging.INFO_LOG_LEVEL)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err := os.MkdirAll(filepath.Join(home, ".config"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".config", "user-dirs.dirs"), []byte("XDG_DESKTOP_DIR=\"$HOME/Desktop\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var changes []filechanges.Change
	env := &Env{ProjRoot: home, AllowDeferredTools: true, RecordFileChanges: func(c []filechanges.Change, _ []filechanges.Commit) { changes = append(changes, c...) }}
	command := `desktop="$(xdg-user-dir DESKTOP 2>/dev/null || printf '%s/Desktop' "$HOME")"; mkdir -p "$desktop" || exit 1; target="$desktop/test.txt"; if [ -e "$target" ]; then n=1; while [ -e "$desktop/test-$n.txt" ]; do n=$((n+1)); done; target="$desktop/test-$n.txt"; fi; printf 'File di test.\n' > "$target" && printf 'Creato: %s\n' "$target"`
	for _, expected := range []string{"Desktop/test.txt", "Desktop/test-1.txt"} {
		changes = nil
		raw, _ := json.Marshal(map[string]any{"command": command, "intent": "reproduce exact Desktop command in isolated home"})
		result, err := Exec(context.Background(), env, "agent", tooling.Invocation{Name: "shell", Args: raw})
		if err != nil {
			t.Fatal(err)
		}
		if len(changes) != 1 || changes[0].Name != expected || changes[0].Added != 1 {
			t.Fatalf("command result=%+v; tracked=%+v; paths=%v", result, changes, ShellFileChangePaths(home, command))
		}
	}
}
