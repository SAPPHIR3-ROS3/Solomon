package server

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/gofrs/flock"
)

// WaitForShutdown waits until cleanup has released the daemon lock. HTTP
// shutdown and removal of state alone do not mean a replacement can start.
func WaitForShutdown(ctx context.Context) error {
	home, err := paths.SolomonHome()
	if err != nil {
		return err
	}
	lock := flock.New(filepath.Join(home, "run", "server", "daemon.lock"))
	defer lock.Close()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		locked, err := lock.TryLock()
		if err != nil {
			return err
		}
		if locked {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for daemon shutdown: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
