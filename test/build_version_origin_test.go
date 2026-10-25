package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOriginTagsAndLocalFallback(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "resolver")
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	build := exec.Command("go", "build", "-o", executable, "./scripts/build_version")
	build.Dir = repositoryRootForTestLayout(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build resolver: %v\n%s", err, out)
	}
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	for _, name := range []string{"origin", "checkout"} {
		dir := filepath.Join(root, name)
		git(root, "init", "--quiet", dir)
		git(dir, "-c", "user.name=Version Test", "-c", "user.email=version@example.invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "--allow-empty", "-m", "initial")
	}
	origin, checkout := filepath.Join(root, "origin"), filepath.Join(root, "checkout")
	git(origin, "tag", "v2026.930.0")
	git(origin, "tag", "v2026.1023.1")
	git(origin, "tag", "v2099.1.0-rc1")
	git(checkout, "tag", "v2099.1.0")
	git(checkout, "remote", "add", "origin", origin)
	verify := func(want string) {
		t.Helper()
		cmd := exec.Command(executable)
		cmd.Dir = checkout
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(string(out)); got != want {
			t.Fatalf("version = %q, want %q", got, want)
		}
	}
	verify("v2026.1023.1-dev")
	git(checkout, "remote", "set-url", "origin", filepath.Join(root, "unreachable"))
	verify("v2099.1.0")
	if err := os.WriteFile(filepath.Join(checkout, "modified"), []byte("dirty"), 0600); err != nil {
		t.Fatal(err)
	}
	verify("v2099.1.0-dev")
}
