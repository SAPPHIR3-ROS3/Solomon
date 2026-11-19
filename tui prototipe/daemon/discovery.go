package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type State struct {
	PID       int       `json:"pid"`
	URL       string    `json:"url"`
	Home      string    `json:"home"`
	StartedAt time.Time `json:"started_at"`
}

type Client struct {
	base string
	http *http.Client
}

func Discover(ctx context.Context, home string) (*Client, error) {
	if home == "" {
		home = strings.TrimSpace(os.Getenv("SOLOMON_HOME"))
		if home == "" {
			h, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			home = filepath.Join(h, ".solomon")
		}
	}
	data, err := os.ReadFile(filepath.Join(home, "run", "server", "state.json"))
	if err != nil {
		return nil, err
	}
	var state State
	if err = json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode daemon state: %w", err)
	}
	u, err := url.Parse(state.URL)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.Port() == "" {
		return nil, errors.New("invalid daemon URL")
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return nil, errors.New("daemon URL must use a loopback IP")
	}
	if state.PID <= 1 || state.StartedAt.IsZero() {
		return nil, errors.New("invalid daemon identity")
	}
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: time.Second}).DialContext, ResponseHeaderTimeout: 10 * time.Second}
	client := &Client{base: strings.TrimRight(state.URL, "/"), http: &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	healthCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(healthCtx, http.MethodGet, client.base+"/health", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("daemon health: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daemon health: %s", resp.Status)
	}
	var health struct {
		OK     bool  `json:"ok"`
		Server State `json:"server"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&health); err != nil {
		return nil, fmt.Errorf("decode daemon health: %w", err)
	}
	if !health.OK || health.Server.PID != state.PID || !health.Server.StartedAt.Equal(state.StartedAt) || health.Server.URL != state.URL {
		return nil, errors.New("daemon health identity mismatch")
	}
	if state.Home != "" && !samePath(state.Home, home) || health.Server.Home != "" && !samePath(health.Server.Home, home) {
		return nil, errors.New("daemon home mismatch")
	}
	return client, nil
}

func samePath(a, b string) bool {
	a, err := filepath.Abs(a)
	if err != nil {
		return false
	}
	b, err = filepath.Abs(b)
	if err != nil {
		return false
	}
	if a == b {
		return true
	}
	x, err := os.Stat(a)
	if err != nil {
		return false
	}
	y, err := os.Stat(b)
	return err == nil && os.SameFile(x, y)
}
