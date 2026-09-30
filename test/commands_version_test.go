package test

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
)

func TestVersionStringReleaseCommit(t *testing.T) {
	// Isolate the process-wide release state from other version tests.
	if os.Getenv("SOLOMON_TEST_RELEASE_VERSION") == "1" {
		const tag = "v2026.1001.1"
		commands.SetEffectiveReleaseVersion(tag, false)
		if got := commands.VersionString(); !strings.HasPrefix(got, tag+"-dev") {
			t.Fatalf("ahead build version = %q, want development suffix", got)
		}
		commands.SetEffectiveReleaseVersion(tag, true)
		if got := commands.VersionString(); got != tag {
			t.Fatalf("identical build version = %q, want %q", got, tag)
		}
		commands.SetEffectiveReleaseVersion(tag, false)
		if got := commands.VersionString(); !strings.HasPrefix(got, tag+"-dev") {
			t.Fatalf("subsequent ahead build version = %q, want development suffix", got)
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestVersionStringReleaseCommit$")
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
