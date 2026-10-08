package lifecycle

import (
	"context"
	"debug/buildinfo"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
	"github.com/gofrs/flock"
)

// HotInstall replaces the local installation while its daemon and native clients
// are stopped, then starts fresh processes in the daemon's previous mode.
func HotInstall(ctx context.Context, target, defaultDevDir string, install func() error) error {
	home, err := paths.SolomonHome()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(home, "run"), 0700); err != nil {
		return err
	}
	lock := flock.New(filepath.Join(home, "run", "update.lock"))
	locked, err := lock.TryLock()
	if err != nil {
		return err
	}
	if !locked {
		return fmt.Errorf("another Solomon update is already running")
	}
	defer lock.Unlock()
	ops := platformRuntimeUpdateOps(ctx, "", serverruntime.State{Mode: "dev", DevDir: defaultDevDir})
	return hotInstall(target, install, ops)
}

func hotInstall(target string, install func() error, ops runtimeUpdateOps) (err error) {
	// A partial stop or failed installation must not silently leave clients down.
	defer func() {
		if err != nil {
			if stopErr := ops.quiesce(); stopErr != nil {
				err = errors.Join(err, stopErr)
				return
			}
			err = errors.Join(err, ops.restore(target))
		}
	}()
	if err = ops.stop(target); err != nil {
		return err
	}
	if err = install(); err != nil {
		return err
	}
	return ops.start(target)
}

func verifyInstalledDaemon(target string, state serverruntime.State) error {
	info, err := buildinfo.ReadFile(target)
	if err != nil {
		return fmt.Errorf("read installed daemon build: %w", err)
	}
	for _, setting := range info.Settings {
		if setting.Key != "-ldflags" {
			continue
		}
		for _, field := range strings.Fields(setting.Value) {
			if _, tree, ok := strings.Cut(field, ".sourceTree="); ok && tree != "" && tree != state.SourceTree {
				return fmt.Errorf("daemon serves source tree %s, installed binary contains %s", state.SourceTree, tree)
			}
		}
	}
	return nil
}

func waitClientRegistration(ctx context.Context, client desktopUpdateProcess) error {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if processIdentity(client.pid) != client.identity {
			return fmt.Errorf("restarted Solomon client %d exited during startup", client.pid)
		}
		clients, err := updateClients(client.executable)
		if err != nil {
			return err
		}
		for _, registered := range clients {
			if registered.pid == client.pid && registered.identity == client.identity && registered.stopFile != "" {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("restarted Solomon client %d did not register", client.pid)
}
