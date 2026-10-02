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
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(state.URL, "/")+"/health", nil)
	if err != nil {
		return false
	}
	response, err := (&http.Client{Timeout: time.Second}).Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false
	}
	var health Health
	return json.NewDecoder(response.Body).Decode(&health) == nil && health.OK && health.Server.PID == state.PID
}
