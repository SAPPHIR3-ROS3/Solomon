package test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
)

func TestVersionStringSourceStamps(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands."
	commit := strings.Repeat("a", 40)
	cleanTree := strings.Repeat("b", 40)
	for _, tc := range []struct {
		name, tree, want string
	}{
		{"unchanged inputs", cleanTree, "v2026.930.0"},
		{"modified inputs", strings.Repeat("c", 40), "v2026.930.0-dev-aaaaaaa-dirty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := "solomon"
			if runtime.GOOS == "windows" {
				name += ".exe"
			}
			binary := filepath.Join(t.TempDir(), name)
			flags := "-X " + prefix + "version=v2026.930.0 -X " + prefix + "commit=" + commit + " -X " + prefix + "sourceTree=" + tc.tree + " -X " + prefix + "commitTree=" + cleanTree
			cmd := exec.Command("go", "build", "-buildvcs=false", "-ldflags="+flags, "-o", binary, "./cmd/solomon")
			cmd.Dir = root
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("build stamped binary: %v\n%s", err, out)
			}
			out, err := exec.Command(binary, "version", "--binary").CombinedOutput()
			if err != nil {
				t.Fatalf("read stamped version: %v\n%s", err, out)
			}
			if !strings.Contains(string(out), "\n"+tc.want+"\n") {
				t.Fatalf("version output = %q, want %q", out, tc.want)
			}
		})
	}
}

func TestVersionStringReleaseSource(t *testing.T) {
	// Isolate the process-wide release state from other version tests.
	if os.Getenv("SOLOMON_TEST_RELEASE_VERSION") == "1" {
		const tag = "v2026.1001.1"
		commands.SetEffectiveReleaseVersion(tag, false)
		if got := commands.VersionString(); !strings.HasPrefix(got, tag+"-dev") {
			t.Fatalf("unverified build version = %q, want development suffix", got)
		}
		commands.SetEffectiveReleaseVersion(tag, true)
		if got := commands.VersionString(); got != tag {
			t.Fatalf("verified release source version = %q, want %q", got, tag)
		}
		commands.SetEffectiveReleaseVersion(tag, false)
		if got := commands.VersionString(); !strings.HasPrefix(got, tag+"-dev") {
			t.Fatalf("subsequent unverified build version = %q, want development suffix", got)
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestVersionStringReleaseSource$")
	cmd.Env = append(os.Environ(), "SOLOMON_TEST_RELEASE_VERSION=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("version regression test: %v\n%s", err, output)
	}
}

func TestVersionStringNonEmpty(t *testing.T) {
	t.Parallel()
	if v := commands.VersionString(); strings.TrimSpace(v) == "" {
		t.Fatal("expected non-empty version")
	}
}

func TestWriteVersion(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	commands.WriteVersion(&buf)
	if strings.TrimSpace(buf.String()) == "" {
		t.Fatal("expected non-empty output")
	}
}
