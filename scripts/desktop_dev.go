//go:build ignore

// desktop_dev starts Wails development against the current Solomon dev server.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func main() {
	state, err := serverruntime.LoadState()
	if err != nil {
		fail("Solomon server is not running; start it with: solomon server start dev <gui-directory>")
	}
	if state.Mode != "dev" || state.Vite != "running" {
		fail("Solomon server is not in GUI development mode; start it with: solomon server start dev <gui-directory>")
	}
	if err := verifyHealth(state.LocalURL); err != nil {
		fail("Solomon server at %s is not healthy: %v", state.LocalURL, err)
	}

	root, err := os.Getwd()
	if err != nil {
		fail("read working directory: %v", err)
	}
	command := exec.Command("wails", "dev", "-frontenddevserverurl", state.LocalURL)
	command.Dir = filepath.Join(root, "gui", "desktop")
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		fail("start Wails: %v", err)
	}
}

func verifyHealth(serverURL string) error {
	response, err := http.Get(serverURL + "/health")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GET /health returned %s", response.Status)
	}
	var health struct {
		OK bool `json:"ok"`
	}
	if err := json.NewDecoder(response.Body).Decode(&health); err != nil {
		return err
	}
	if !health.OK {
		return fmt.Errorf("GET /health reported not ready")
	}
	return nil
}

func fail(format string, values ...any) {
	fmt.Fprintf(os.Stderr, "desktop-dev: "+format+"\n", values...)
	os.Exit(1)
}
