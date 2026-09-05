package codex

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

// CodexVersionRegistryURL returns metadata for npm's stable latest tag.
var CodexVersionRegistryURL = "https://registry.npmjs.org/@openai%2Fcodex/latest"

const clientVersionTTL = 6 * time.Hour

var stableClientVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

type clientVersionDocument struct {
	Version   string    `json:"version"`
	CheckedAt time.Time `json:"checked_at"`
}

var clientVersionCache struct {
	sync.Mutex
	path       string
	registry   string
	document   clientVersionDocument
	retryAfter time.Time
}

// ResolveClientVersion checks npm at most once per six hours, unless forced.
// Registry failures preserve the cached version, or the built-in fallback on
// first use, and are retried after five minutes. No OAuth credentials go to npm.
func ResolveClientVersion(ctx context.Context, force bool) string {
	if ctx == nil {
		ctx = context.Background()
	}
	home, _ := paths.SolomonHome()
	cachePath := ""
	if home != "" {
		cachePath = filepath.Join(home, "codex-client-version.json")
	}
	clientVersionCache.Lock()
	defer clientVersionCache.Unlock()
	c := &clientVersionCache
	if c.path != cachePath || c.registry != CodexVersionRegistryURL || c.document.Version == "" {
		c.path, c.registry = cachePath, CodexVersionRegistryURL
		c.document = clientVersionDocument{Version: ClientVersion}
		c.retryAfter = time.Time{}
		if data, err := os.ReadFile(cachePath); err == nil {
			var saved clientVersionDocument
			if json.Unmarshal(data, &saved) == nil && stableClientVersion.MatchString(saved.Version) {
				c.document = saved
			}
		}
	}
	now := time.Now()
	age := now.Sub(c.document.CheckedAt)
	if !force && ((!c.document.CheckedAt.IsZero() && age >= 0 && age < clientVersionTTL) || now.Before(c.retryAfter)) {
		return c.document.Version
	}
	c.retryAfter = now.Add(5 * time.Minute)
	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(lookupCtx, http.MethodGet, CodexVersionRegistryURL, nil)
	if err != nil {
		return c.document.Version
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return c.document.Version
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return c.document.Version
	}
	var metadata struct {
		Version string `json:"version"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&metadata) != nil || !stableClientVersion.MatchString(metadata.Version) {
		return c.document.Version
	}
	c.document = clientVersionDocument{Version: metadata.Version, CheckedAt: time.Now()}
	c.retryAfter = time.Time{}
	saveClientVersion(cachePath, c.document)
	return c.document.Version
}

func saveClientVersion(path string, document clientVersionDocument) {
	if path == "" {
		return
	}
	if os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	data, err := json.Marshal(document)
	if err != nil {
		return
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".codex-client-version-*")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr == nil && closeErr == nil {
		_ = os.Rename(file.Name(), path)
	}
}

func clientUserAgent(version string) string {
	return "codex_cli_rs/" + version + " (Ubuntu 22.04.0; x86_64) WindowsTerminal"
}
