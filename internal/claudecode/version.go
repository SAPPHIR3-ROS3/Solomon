package claudecode

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

const FallbackVersion = "2.1.75"

const githubLatestReleaseURL = "https://api.github.com/repos/anthropics/claude-code/releases/latest"

const versionCacheTTL = 24 * time.Hour

type versionCache struct {
	Version   string    `json:"version"`
	FetchedAt time.Time `json:"fetched_at"`
}

var (
	versionMu      sync.Mutex
	resolvedVersion string
)

func Version() string {
	versionMu.Lock()
	defer versionMu.Unlock()
	if resolvedVersion != "" {
		return resolvedVersion
	}
	resolvedVersion = resolveVersionLocked()
	return resolvedVersion
}

func resolveVersionLocked() string {
	if entry, ok := readVersionCache(); ok {
		if time.Since(entry.FetchedAt) < versionCacheTTL && normalizeVersion(entry.Version) != "" {
			return normalizeVersion(entry.Version)
		}
	}
	if v, err := fetchLatestFromGitHub(); err == nil {
		if norm := normalizeVersion(v); norm != "" {
			_ = writeVersionCache(versionCache{Version: norm, FetchedAt: time.Now()})
			return norm
		}
	}
	if entry, ok := readVersionCache(); ok {
		if norm := normalizeVersion(entry.Version); norm != "" {
			return norm
		}
	}
	return FallbackVersion
}

func fetchLatestFromGitHub() (string, error) {
	cli := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, githubLatestReleaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Solomon")
	resp, err := cli.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github releases: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.TagName) == "" {
		return "", fmt.Errorf("github releases: empty tag_name")
	}
	return payload.TagName, nil
}

func normalizeVersion(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "v")
	tag = strings.TrimPrefix(tag, "V")
	if tag == "" || tag[0] < '0' || tag[0] > '9' {
		return ""
	}
	return tag
}

func versionCachePath() (string, error) {
	root, err := paths.SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "cache", "claude-code-version.json"), nil
}

func readVersionCache() (versionCache, bool) {
	path, err := versionCachePath()
	if err != nil {
		return versionCache{}, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return versionCache{}, false
	}
	var entry versionCache
	if err := json.Unmarshal(raw, &entry); err != nil {
		return versionCache{}, false
	}
	return entry, true
}

func writeVersionCache(entry versionCache) error {
	path, err := versionCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func ResetVersionCacheForTest() {
	versionMu.Lock()
	defer versionMu.Unlock()
	resolvedVersion = ""
}
