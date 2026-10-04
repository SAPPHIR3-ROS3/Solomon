//go:build !windows

package main

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func restartTUIAfterDaemonUpdate(args []string, previousPID int) error {
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		state, err := serverruntime.LoadState()
		if err == nil && state.PID != previousPID {
			health, err := serverruntime.ReadHealth(context.Background(), state)
			if err == nil && health.OK && health.Server.PID == state.PID {
				// Re-enter through the freshly installed CLI. All PTYs will be
				// recreated by the new daemon; the client's output cursor resets.
				executable, err := os.Executable()
				if err != nil {
					return err
				}
				return syscall.Exec(executable, args, os.Environ())
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("Solomon daemon did not restart; inspect ~/.solomon/logs/update/update.log")
}
