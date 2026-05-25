package test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/agent/commands"
)

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
