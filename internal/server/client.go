package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// EnsureRunning reuses a healthy local daemon or starts it through the CLI.
// cliPath must point to solomon, since the daemon also launches CLI PTY children.
func EnsureRunning(ctx context.Context, cliPath string) (State, error) {
	if state, err := LoadState(); err == nil && daemonHealthy(ctx, state) {
		return state, nil
	}
	command := exec.CommandContext(ctx, cliPath, "server", "start")
	output, err := command.CombinedOutput()
	// The CLI's lifecycle commands may print failures without a nonzero exit code.
	// Readiness, rather than the command exit status, is the final authority.
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
	response, err := (&http.Client{Timeout: time.Second}).Do(request)
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
	state, err := LoadState()
	if err != nil {
		return ""
	}
	health, err := ReadHealth(context.Background(), state)
	if err != nil || !health.OK || health.Server.PID != state.PID {
		return ""
	}
	return health.Server.Version
}
