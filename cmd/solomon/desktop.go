package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func prepareDesktop() (string, error) {
	logging.LogInit(logging.INFO_LOG_LEVEL)
	cli, err := os.Executable()
	if err != nil {
		return "", err
	}
	return updater.EnsureDesktop(context.Background(), commands.LocalVersionString(), cli, os.Stdout)
}

func desktopSetupRequired(args []string) bool {
	if len(args) < 2 {
		return true
	}
	switch args[1] {
	case "version":
		return len(args) < 3 || args[2] != "--binary"
	case "server", "sandbox-worker", "desktop":
		return false
	default:
		return true
	}
}

func launchDesktop(args []string) error {
	if len(args) > 0 && args[0] == "install" {
		return installDesktopCLI(args[1:])
	}
	if len(args) != 0 {
		return fmt.Errorf("usage: solomon desktop [install [release-tag]]")
	}
	path, err := prepareDesktop()
	if err != nil {
		return err
	}
	cmd := exec.Command(path)
	configureDesktopProcess(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start Solomon Desktop: %w", err)
	}
	return cmd.Process.Release()
}

func installDesktopCLI(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: solomon desktop install [release-tag]")
	}
	logging.LogInit(logging.INFO_LOG_LEVEL)
	ctx := context.Background()
	tag := "latest"
	if len(args) == 1 {
		tag = args[0]
	}
	if tag == "latest" {
		result := updater.Check(ctx, "dev")
		if result.Err != nil {
			return result.Err
		}
		tag = result.LatestTag
	}
	cliPath, err := os.Executable()
	if err != nil {
		return err
	}
	if err := updater.InstallDesktop(ctx, tag, cliPath, os.Stdout); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "Solomon Desktop installed. Open it from the application menu or run: solomon desktop")
	return nil
}
