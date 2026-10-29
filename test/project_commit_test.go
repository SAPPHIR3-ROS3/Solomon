package test

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProjectCommitStagedOnlyAndFailures(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("init")
	git("config", "user.name", "Commit Test")
	git("config", "user.email", "commit@example.com")
	git("config", "commit.gpgsign", "false")
	write("tracked.txt", "initial")
	write("other.txt", "initial")
	git("add", ".")
	git("commit", "-m", "initial")
	endpoint := strings.TrimSuffix(projectFileOperationEndpoint(t, root), "/file-operation") + "/commit"
	commit := func(message string, want int) map[string]any {
		t.Helper()
		response := postJSONForServerTest(t, endpoint, map[string]string{"message": message})
		defer response.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("commit status %d, want %d: %v", response.StatusCode, want, payload)
		}
		return payload
	}
	initial := git("rev-parse", "HEAD")
	commit("   ", http.StatusBadRequest)
	commit("empty index", http.StatusConflict)
	write("tracked.txt", "staged version")
	git("add", "tracked.txt")
	write("tracked.txt", "unstaged version")
	write("other.txt", "unstaged other")
	write("untracked.txt", "untracked")
	message := "--literal commit message\n\nBody with $() and quotes ' \""
	if result := commit(message, http.StatusOK); result["committed"] != true {
		t.Fatalf("response: %v", result)
	}
	if git("show", "HEAD:tracked.txt") != "staged version" || git("show", "HEAD:other.txt") != "initial" {
		t.Fatal("committed unstaged content")
	}
	if git("log", "-1", "--format=%B") != message {
		t.Fatal("commit message was not passed literally")
	}
	if git("diff", "--cached", "--name-only") != "" {
		t.Fatal("index not cleared")
	}
	for name, want := range map[string]string{"tracked.txt": "unstaged version", "other.txt": "unstaged other", "untracked.txt": "untracked"} {
		got, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(got) != want {
			t.Fatalf("worktree %s changed: %q, %v", name, got, err)
		}
	}
	if git("rev-parse", "HEAD") == initial {
		t.Fatal("HEAD did not advance")
	}
	git("add", "tracked.txt")
	head := git("rev-parse", "HEAD")
	git("config", "user.name", "")
	git("config", "user.email", "")
	if errText, _ := commit("missing identity", http.StatusConflict)["error"].(string); !strings.Contains(errText, "identity") && !strings.Contains(errText, "empty ident") {
		t.Fatalf("missing identity detail: %q", errText)
	}
	if git("rev-parse", "HEAD") != head || git("diff", "--cached", "--name-only") != "tracked.txt" {
		t.Fatal("failure changed HEAD or index")
	}
	git("config", "user.name", "Commit Test")
	git("config", "user.email", "commit@example.com")
	if runtime.GOOS != "windows" {
		hookDir := filepath.Join(root, "hooks")
		if err := os.Mkdir(hookDir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(hookDir, "pre-commit"), []byte("#!/bin/sh\necho 'blocked by test hook' >&2\nexit 1\n"), 0700); err != nil {
			t.Fatal(err)
		}
		git("config", "core.hooksPath", hookDir)
		result := commit("blocked", http.StatusConflict)
		if !strings.Contains(result["error"].(string), "blocked by test hook") {
			t.Fatalf("hook detail lost: %v", result)
		}
		if git("rev-parse", "HEAD") != head || git("diff", "--cached", "--name-only") != "tracked.txt" {
			t.Fatal("hook failure changed HEAD or index")
		}
	}
}

func TestProjectCommitInitialRepository(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"init"}, {"config", "user.name", "Test"}, {"config", "user.email", "test@example.com"}, {"config", "commit.gpgsign", "false"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "first.txt"), []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", root, "add", "first.txt").Run(); err != nil {
		t.Fatal(err)
	}
	endpoint := strings.TrimSuffix(projectFileOperationEndpoint(t, root), "/file-operation") + "/commit"
	response := postJSONForServerTest(t, endpoint, map[string]string{"message": "initial"})
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("initial commit status %d", response.StatusCode)
	}
	if out, err := exec.Command("git", "-C", root, "show", "HEAD:first.txt").Output(); err != nil || string(out) != "first" {
		t.Fatalf("initial commit missing: %q, %v", out, err)
	}
}
