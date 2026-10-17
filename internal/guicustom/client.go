// Package guicustom lets chat tools configure the daemon without importing its
// runtime (which itself executes chat tools).
package guicustom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

func Request(ctx context.Context, mode, directory string) (map[string]any, error) {
	home, err := paths.SolomonHome()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(home, "run", "server", "state.json"))
	if err != nil {
		return nil, fmt.Errorf("GUI customization needs a running Solomon daemon: %w", err)
	}
	var state struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	address, err := url.Parse(state.URL)
	if err != nil || address.Scheme != "http" || address.Hostname() != "127.0.0.1" || address.User != nil {
		return nil, fmt.Errorf("invalid local daemon address")
	}
	method := http.MethodPost
	if mode == "status" {
		method = http.MethodGet
	}
	body, _ := json.Marshal(map[string]string{"mode": mode, "directory": directory})
	request, err := http.NewRequestWithContext(ctx, method, state.URL+"/__solomon/development", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 11 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err = io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GUI customization: %s: %s", response.Status, data)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	result["restart_required"] = false
	result["next_step"] = "Edit the returned dev_directory sources using the file tools. Vite applies changes through hot reload. Do not stop or restart the daemon."
	if mode == "normal" {
		result["next_step"] = "Bundled interface restored. Custom sources are preserved; reload the interface when ready."
	}
	return result, nil
}
