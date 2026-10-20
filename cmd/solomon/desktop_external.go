//go:build !desktop

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

func launchDesktopWindow() error {
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
