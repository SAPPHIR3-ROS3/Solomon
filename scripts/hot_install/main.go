// hot_install coordinates Make's source installation with native client shutdown.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/lifecycle"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: hot_install <solomon-binary> <make> <bin-directory>")
		os.Exit(1)
	}
	target, err := filepath.Abs(os.Args[1])
	if err != nil {
		fail(err)
	}
	root, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	run := func(name string, args ...string) error {
		command := exec.CommandContext(ctx, name, args...)
		command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
		return command.Run()
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		fail(err)
	}
	stage, err := os.MkdirTemp(filepath.Dir(target), ".solomon-install-*")
	if err != nil {
		fail(err)
	}
	// Return normally through this helper so deferred cleanup also runs on errors.
	if err := install(ctx, target, root, stage, run); err != nil {
		_ = os.RemoveAll(stage)
		fail(err)
	}
	_ = os.RemoveAll(stage)
	fmt.Println("Solomon daemon and previously open native clients restarted.")
}

func install(ctx context.Context, target, root, stage string, run func(string, ...string) error) error {
	stagedCLI := filepath.Join(stage, filepath.Base(target))
	// Compilers, npm, browser downloads and configuration validation complete
	// while the installed daemon and its clients are still available.
	if err := run(os.Args[2], "install-prepare", "OUT="+stagedCLI); err != nil {
		return fmt.Errorf("prepare installation (running service preserved): %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	cursorRoot := strings.TrimSpace(os.Getenv("SOLOMON_CURSOR_API_ROOT"))
	if cursorRoot == "" {
		home, err := paths.SolomonHome()
		if err != nil {
			return err
		}
		cursorRoot = filepath.Join(home, "integrations", "cursor")
	}
	if err := os.MkdirAll(filepath.Dir(cursorRoot), 0755); err != nil {
		return err
	}
	cursorStage, err := os.MkdirTemp(filepath.Dir(cursorRoot), ".cursor-install-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(cursorStage)
	if err := run("go", "run", "scripts/cursor_bundler.go", "prepare-install", cursorStage); err != nil {
		return fmt.Errorf("prepare Cursor dependencies (running service preserved): %w", err)
	}
	desktop := filepath.Join(root, "gui", "desktop", "build", "bin", "solomon-desktop")
	if runtime.GOOS == "windows" {
		desktop += ".exe"
	} else if runtime.GOOS == "darwin" {
		desktop = filepath.Join(filepath.Dir(desktop), "Solomon.app")
	}
	version := os.Getenv("VERSION")
	if version == "" {
		version = "dev"
	}
	return lifecycle.HotInstall(ctx, target, filepath.Join(root, "gui"), func() error {
		if err := updater.CommitInstall(stagedCLI, target); err != nil {
			return err
		}
		if err := updater.InstallBuiltDesktop(ctx, version, target, desktop, os.Stdout); err != nil {
			return err
		}
		return run(os.Args[2], "install-deploy", "BIN_DIR="+os.Args[3], "CURSOR_STAGE="+cursorStage)
	})
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "hot-install failed:", err)
	os.Exit(1)
}
