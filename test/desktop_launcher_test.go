package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopLauncher(t *testing.T) {
	dir := t.TempDir()
	cli := filepath.Join(dir, "solomon.exe")
	build := exec.Command("go", "build", "-o", cli, "./cmd/solomon")
	build.Dir = ".."
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build launcher: %v\n%s", err, output)
	}
	checkError := func(want string, args ...string) {
		t.Helper()
		cmd := exec.Command(cli, append([]string{"desktop"}, args...)...)
		cmd.Env = append(os.Environ(), "SOLOMON_HOME="+filepath.Join(dir, "home"))
		output, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(output), want) {
			t.Fatalf("desktop %v = %v, %s; want error containing %q", args, err, output, want)
		}
	}
	checkError("usage: solomon desktop", "extra")
	checkError("usage: solomon desktop install", "install", "one", "two")
	if _, err := os.Stat(filepath.Join(dir, "home")); !os.IsNotExist(err) {
		t.Fatal("desktop launcher should not enter terminal onboarding or initialize user data")
	}
}
