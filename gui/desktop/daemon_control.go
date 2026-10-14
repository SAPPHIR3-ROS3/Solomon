package main

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

var daemonControlMu sync.Mutex

func (b ServerBridge) DaemonRunning() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := serverruntime.LoadRunningState(ctx)
	return err == nil
}

// This binding stays available when the HTTP daemon is down.
func (b ServerBridge) SetDaemonEnabled(enabled bool) (bool, error) {
	daemonControlMu.Lock()
	defer daemonControlMu.Unlock()
	if enabled {
		if err := ensureDesktopServer(); err != nil {
			return b.DaemonRunning(), err
		}
	} else if b.DaemonRunning() {
		cli, err := desktopCLI()
		if err != nil {
			return true, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		output, err := exec.CommandContext(ctx, cli, "server", "stop").CombinedOutput()
		if err != nil {
			return b.DaemonRunning(), fmt.Errorf("stop daemon: %w: %s", err, output)
		}
	}
	running := b.DaemonRunning()
	if running != enabled {
		return running, fmt.Errorf("daemon did not reach the requested state")
	}
	return running, nil
}
