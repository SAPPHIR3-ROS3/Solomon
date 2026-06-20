package test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/skills"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func TestBtwInHelpRegistry(t *testing.T) {
	t.Parallel()
	found := false
	for _, row := range commands.Registry(&config.Root{}) {
		if len(row) > 0 && strings.Contains(row[0], "/btw") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("/btw missing from help registry")
	}
}

func TestBtwIdleDispatchInfoMessage(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	termcolor.Init(termcolor.InitOptions{Out: &buf, ForceColor: false})
	d := commands.Deps{Out: &buf, Cfg: &config.Root{}}
	if err := commands.Btw(d, []string{"/btw"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "during generation") {
		t.Fatalf("expected idle info message, got %q", buf.String())
	}
}

func TestBtwReservedSlashName(t *testing.T) {
	t.Parallel()
	if _, ok := skills.ReservedSlashCommandNames()["btw"]; !ok {
		t.Fatal("btw should be reserved")
	}
}
