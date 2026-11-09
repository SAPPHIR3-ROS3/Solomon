package test

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

func baselineGitFixture(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git fixture: %v: %s", err, output)
	}
}

func baselineEndpoint(t *testing.T, root string) string {
	t.Helper()
	return strings.TrimSuffix(projectFileOperationEndpoint(t, root), "/file-operation") + "/file?revision=HEAD&path="
}

func readBaselineFixture(t *testing.T, endpoint, name, want string, repo bool) {
	t.Helper()
	response, err := http.Get(endpoint + url.QueryEscape(name))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("baseline status: %d", response.StatusCode)
	}
	var payload struct {
		Content string
		IsRepo  bool
	}
	decodeServerTestJSON(t, response, &payload)
	if payload.Content != want || payload.IsRepo != repo {
		t.Fatalf("unexpected baseline: %+v", payload)
	}
}

func TestEditorBaselineExpectedAbsenceDoesNotLogFailures(t *testing.T) {
	read := diagnosticLogs(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	endpoint := baselineEndpoint(t, root)
	readBaselineFixture(t, endpoint, "new.txt", "", false)
	baselineGitFixture(t, root, "init", "-b", "main")
	readBaselineFixture(t, endpoint, "new.txt", "", true)
	for _, name := range []string{"tracked.txt", "literal[1].txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	baselineGitFixture(t, root, "add", "--", "tracked.txt", "literal[1].txt")
	baselineGitFixture(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "Fixture")
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		readBaselineFixture(t, endpoint, "new.txt", "", true)
	}
	baselineGitFixture(t, root, "add", "new.txt")
	readBaselineFixture(t, endpoint, "new.txt", "", true)
	readBaselineFixture(t, endpoint, "literal[1].txt", "literal[1].txt", true)
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "file.txt"), []byte("nested"), 0600); err != nil {
		t.Fatal(err)
	}
	baselineGitFixture(t, root, "add", ".")
	baselineGitFixture(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "Nested fixture")
	nested := baselineEndpoint(t, filepath.Join(root, "nested"))
	readBaselineFixture(t, nested, "file.txt", "nested", true)
	if text := read(); strings.Contains(text, "outcome:failed") || strings.Contains(text, "exit status 128") {
		t.Fatalf("expected absence generated a failure: %s", text)
	}
}

func TestEditorBaselineCancellationDoesNotCreateErrorNotification(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	read := diagnosticLogs(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	baselineGitFixture(t, root, "init", "-b", "main")
	baselineGitFixture(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "Fixture")
	endpoint := baselineEndpoint(t, root)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	marker := filepath.Join(bin, "started")
	script := "#!/bin/sh\nfor arg in \"$@\"; do\n if [ \"$arg\" = ls-tree ]; then touch \"$SOLOMON_BASELINE_MARKER\"; exec /bin/sleep 20; fi\ndone\nexec \"$SOLOMON_BASELINE_GIT\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SOLOMON_BASELINE_GIT", git)
	t.Setenv("SOLOMON_BASELINE_MARKER", marker)
	before, err := logging.ReadNotifications()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"new.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		response, err := http.DefaultClient.Do(request)
		if response != nil {
			response.Body.Close()
		}
		done <- err
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Git fixture did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-done; err == nil {
		t.Fatal("request was not canceled")
	}
	for !strings.Contains(read(), "editor baseline request canceled") {
		if time.Now().After(deadline) {
			t.Fatal("cancellation was not observed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	after, err := logging.ReadNotifications()
	if err != nil || len(after) != len(before) {
		t.Fatalf("cancellation generated error notification: before=%d after=%d err=%v", len(before), len(after), err)
	}
}

func TestEditorBaselineRealGitFailuresRemainVisible(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	for _, script := range []string{"exit 128", "kill -KILL $$"} {
		t.Run(script, func(t *testing.T) {
			read := diagnosticLogs(t)
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("file"), 0600); err != nil {
				t.Fatal(err)
			}
			endpoint := baselineEndpoint(t, root)
			bin := t.TempDir()
			if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\n"+script+"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			response, err := http.Get(endpoint + "file.txt")
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusInternalServerError {
				t.Fatalf("Git failure was swallowed: %d", response.StatusCode)
			}
			if text := read(); !strings.Contains(text, "outcome:failed") || !strings.Contains(text, "operation:editor.baseline.rev-parse") {
				t.Fatalf("Git failure was not logged: %s", text)
			}
		})
	}
}
