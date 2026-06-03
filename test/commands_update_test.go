package test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func TestUpdateDoesNotInstall(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	var installed bool
	on := false
	cfg := &config.Root{AutoUpdate: &on}
	err := commands.Update(commands.Deps{
		Out: &buf,
		Cfg: cfg,
		CheckForUpdate: func(force bool) (*updater.Notice, error) {
			return &updater.Notice{Current: "v1", Latest: "v2"}, nil
		},
		InstallUpdate: func(tag string) error {
			installed = true
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if installed {
		t.Fatal("expected /update not to install")
	}
	out := buf.String()
	if !strings.Contains(out, "/autoupdate") {
		t.Fatalf("expected autoupdate hint, got %q", out)
	}
	if !strings.Contains(out, "/upgrade") {
		t.Fatalf("expected upgrade hint, got %q", out)
	}
}

func TestUpdateUpToDateKeepsScreen(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	banner := false
	err := commands.Update(commands.Deps{
		Out: &buf,
		CheckForUpdate: func(force bool) (*updater.Notice, error) {
			return nil, nil
		},
		PrintWelcomeBanner: func() { banner = true },
	})
	if err != nil {
		t.Fatal(err)
	}
	if banner {
		t.Fatal("expected no welcome banner reprint when up to date")
	}
	out := buf.String()
	if strings.Contains(out, "\033[2J") {
		t.Fatal("expected no screen clear when up to date")
	}
	if !strings.Contains(out, "up to date") {
		t.Fatalf("expected up-to-date message, got %q", out)
	}
}

func TestAutoUpdateSave(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	cfg := &config.Root{}
	saved := false
	err := commands.AutoUpdate(commands.Deps{
		Out:     &buf,
		Cfg:     cfg,
		SaveCfg: func() error { saved = true; return nil },
	}, []string{"autoupdate", "on"})
	if err != nil {
		t.Fatal(err)
	}
	if !saved || cfg.AutoUpdate == nil || !*cfg.AutoUpdate {
		t.Fatal("expected autoupdate on in config")
	}
}

func TestUpgradeUpToDateKeepsScreen(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	banner := false
	err := commands.Upgrade(commands.Deps{
		Out: &buf,
		CheckForUpdate: func(force bool) (*updater.Notice, error) {
			return nil, nil
		},
		PrintWelcomeBanner: func() { banner = true },
		InstallUpdate:      func(string) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if banner {
		t.Fatal("expected no welcome banner reprint when up to date")
	}
	out := buf.String()
	if strings.Contains(out, "\033[2J") {
		t.Fatal("expected no screen clear when up to date")
	}
	if !strings.Contains(out, "up to date") {
		t.Fatalf("expected up-to-date message, got %q", out)
	}
}

func TestUpgradeInstalls(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	var tag string
	err := commands.Upgrade(commands.Deps{
		Out: &buf,
		CheckForUpdate: func(force bool) (*updater.Notice, error) {
			return &updater.Notice{Current: "v1", Latest: "v2099.1.0"}, nil
		},
		InstallUpdate: func(latest string) error {
			tag = latest
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v2099.1.0" {
		t.Fatalf("tag %q", tag)
	}
}
