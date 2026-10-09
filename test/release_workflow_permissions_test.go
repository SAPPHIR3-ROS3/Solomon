package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseWorkflowPermissions(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if runtime.GOOS == "windows" {
		bash = filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe")
		_, err = os.Stat(bash)
	}
	if err != nil {
		t.Skip("Bash unavailable")
	}
	root := t.TempDir()
	git := func(dir string, args ...string) string {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir = dir
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git(root, "init")
	git(root, "config", "user.name", "Release test")
	git(root, "config", "user.email", "release@example.invalid")
	branch := git(root, "symbolic-ref", "--short", "HEAD")
	workflow := filepath.Join(root, ".github", "workflows", "release.yml")
	if err := os.MkdirAll(filepath.Dir(workflow), 0700); err != nil {
		t.Fatal(err)
	}
	write := func(path, data string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	scriptData, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "check_release_workflow_permissions.sh"))
	if err != nil {
		t.Fatal(err)
	}
	// Windows checkouts can convert shell scripts to CRLF. Run the LF form
	// used by the Ubuntu release jobs while preserving the script's contents.
	script := filepath.Join(t.TempDir(), "check-release-workflow-permissions.sh")
	write(script, strings.ReplaceAll(string(scriptData), "\r\n", "\n"))
	write(workflow, "old workflow\n")
	git(root, "add", ".")
	git(root, "commit", "-m", "initial workflow")
	old := git(root, "rev-parse", "HEAD")
	write(workflow, "new workflow\n")
	git(root, "add", ".")
	git(root, "commit", "-m", "updated workflow")
	current := git(root, "rev-parse", "HEAD")
	write(filepath.Join(root, "source.go"), "package fixture\n")
	git(root, "add", ".")
	git(root, "commit", "-m", "application changes")

	for _, testCase := range []struct {
		name, sha, token string
		wantFailure      bool
	}{
		{"matching-workflows", current, "false", false},
		{"outdated-workflows", old, "false", true},
		{"explicit-release-token", old, "true", false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			checkout := t.TempDir()
			git(checkout, "init")
			git(checkout, "remote", "add", "origin", root)
			git(checkout, "fetch", "origin", branch)
			// Emulate Actions' detached checkout without switching any branch.
			write(filepath.Join(checkout, ".git", "HEAD"), testCase.sha+"\n")
			command := exec.Command(bash, filepath.ToSlash(script))
			command.Dir = checkout
			command.Env = append(os.Environ(), "DEFAULT_BRANCH="+branch, "RELEASE_TOKEN_CONFIGURED="+testCase.token)
			output, err := command.CombinedOutput()
			if (err != nil) != testCase.wantFailure {
				t.Fatalf("permission preflight: %v\n%s", err, output)
			}
			if testCase.wantFailure && !strings.Contains(string(output), "RELEASE_TOKEN") {
				t.Fatalf("missing actionable permission error: %s", output)
			}
		})
	}
}
