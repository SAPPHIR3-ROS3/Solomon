package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

// LoadRunningState recovers lost startup metadata from the configured local
// daemon. A home match prevents one installation from adopting another's daemon.
func LoadRunningState(ctx context.Context) (State, error) {
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	state, stateErr := LoadState()
	if stateErr == nil && localDaemonURL(state.URL) {
		health, err := ReadHealth(ctx, state)
		if err == nil && health.OK && health.Server.PID == state.PID && health.Server.StartedAt.Equal(state.StartedAt) {
			home, err := paths.SolomonHome()
			if err != nil {
				return State{}, err
			}
			if health.Server.Home == "" || sameSolomonHome(health.Server.Home, home) {
				return health.Server, nil
			}
		}
	}
	port, err := config.ConfiguredServerPort()
	if err != nil {
		return State{}, err
	}
	address := "http://127.0.0.1:" + strconv.Itoa(port)
	health, err := ReadHealth(ctx, State{URL: address})
	if ctx.Err() != nil {
		return State{}, ctx.Err()
	}
	if err == nil {
		home, homeErr := paths.SolomonHome()
		if homeErr != nil {
			return State{}, homeErr
		}
		home, homeErr = filepath.Abs(home)
		if homeErr != nil {
			return State{}, homeErr
		}
		if !health.OK || health.Server.PID <= 1 || health.Server.StartedAt.IsZero() || health.Server.URL != address || health.Server.Home == "" || !sameSolomonHome(health.Server.Home, home) {
			return State{}, fmt.Errorf("port %d responds but does not identify a Solomon daemon for %s", port, home)
		}
		if err := SaveState(health.Server); err != nil {
			return State{}, fmt.Errorf("recover daemon state: %w", err)
		}
		return health.Server, nil
	}
	if stateErr != nil && !errors.Is(stateErr, os.ErrNotExist) {
		return State{}, stateErr
	}
	return State{}, os.ErrNotExist
}

func sameSolomonHome(a, b string) bool {
	a, _ = filepath.Abs(a)
	b, _ = filepath.Abs(b)
	if a == b || (runtime.GOOS == "windows" && strings.EqualFold(a, b)) {
		return true
	}
	first, err := os.Stat(a)
	if err != nil {
		return false
	}
	second, err := os.Stat(b)
	return err == nil && os.SameFile(first, second)
}

func localDaemonURL(address string) bool {
	parsed, err := url.Parse(address)
	return err == nil && parsed.Scheme == "http" && (parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "localhost" || parsed.Hostname() == "::1") && parsed.User == nil
}

// EnsureRunning reuses a healthy local daemon or starts it through the CLI.
// cliPath must point to solomon, since the daemon also launches CLI PTY children.
func EnsureRunning(ctx context.Context, cliPath string) (State, error) {
	if state, err := LoadRunningState(ctx); err == nil {
		return state, nil
	}
	command := exec.CommandContext(ctx, cliPath, "server", "start")
	output, err := command.CombinedOutput()
	// Verify readiness even after the CLI reports a successful start.
	if state, stateErr := LoadState(); stateErr == nil && daemonHealthy(ctx, state) {
		return state, nil
	}
	return State{}, fmt.Errorf("Solomon daemon did not become ready (%v): %s; inspect with solomon server logs", err, strings.TrimSpace(string(output)))
}

func daemonHealthy(ctx context.Context, state State) bool {
	health, err := ReadHealth(ctx, state)
	return err == nil && health.OK && health.Server.PID == state.PID
}

// ReadHealth reads live daemon metadata rather than the persisted startup snapshot.
func ReadHealth(ctx context.Context, state State) (Health, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(state.URL, "/")+"/health", nil)
	if err != nil {
		return Health{}, err
	}
	response, err := (&http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(request)
	if err != nil {
		return Health{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Health{}, fmt.Errorf("daemon health: %s", response.Status)
	}
	var health Health
	err = json.NewDecoder(response.Body).Decode(&health)
	return health, err
}

func RunningVersion() string {
	state, err := LoadRunningState(context.Background())
	if err != nil {
		return ""
	}
	health, err := ReadHealth(context.Background(), state)
	if err != nil || !health.OK || health.Server.PID != state.PID {
		return ""
	}
	return health.Server.Version
}
