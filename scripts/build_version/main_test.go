package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionResolverProcess(t *testing.T) {
	if os.Getenv("SOLOMON_VERSION_TEST_PROCESS") == "1" {
		main()
		os.Exit(0)
	}
}

func TestOriginTagsAndLocalFallback(t *testing.T) {
	root := t.TempDir()
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
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	verify := func(want string) {
		t.Helper()
		cmd := exec.Command(executable, "-test.run=^TestVersionResolverProcess$")
		cmd.Dir = checkout
		cmd.Env = append(os.Environ(), "SOLOMON_VERSION_TEST_PROCESS=1")
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
