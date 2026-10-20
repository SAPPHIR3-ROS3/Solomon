package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func desktopCLI() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	base := strings.ToLower(filepath.Base(executable))
	if base == "solomon" || base == "solomon.exe" {
		if configured := os.Getenv("SOLOMON_BINARY"); configured != "" {
			return configured, nil
		}
		return executable, nil
	}
	name := "solomon"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	cli := filepath.Join(filepath.Dir(executable), name)
	if configured := os.Getenv("SOLOMON_BINARY"); configured != "" {
		cli = configured
	} else if _, err := os.Stat(cli); err != nil {
		cli, err = exec.LookPath(name)
		if err != nil {
			return "", fmt.Errorf("Solomon CLI is missing: install %s next to solomon-desktop or set SOLOMON_BINARY", name)
		}
	}
	return cli, nil
}

func ensureDesktopServer() error {
	cli, err := desktopCLI()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err = serverruntime.EnsureRunning(ctx, cli)
	return err
}
