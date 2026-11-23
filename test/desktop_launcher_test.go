package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	if runtime.GOOS == "linux" {
		t.Run("version does not register a desktop launcher", func(t *testing.T) {
			desktop := filepath.Join(dir, "solomon-desktop")
			if err := os.WriteFile(desktop, []byte("desktop fixture"), 0700); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"version"}, {"version", "--binary"}} {
				cmd := exec.Command(cli, args...)
				cmd.Env = append(os.Environ(), "SOLOMON_HOME="+filepath.Join(dir, "version-home"), "XDG_DATA_HOME="+filepath.Join(dir, "xdg-data"))
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("%v: %v\n%s", args, err, output)
				}
			}
			for _, file := range []string{desktop + ".install.json", filepath.Join(dir, "xdg-data", "applications", "solomon.desktop")} {
				if _, err := os.Stat(file); !os.IsNotExist(err) {
					t.Fatalf("version changed desktop registration at %s: %v", file, err)
				}
			}
		})
	}

}
