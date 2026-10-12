package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/gofrs/flock"
)

// This is a local backoff, not an estimate of the provider's quota reset.
const internalRateLimitCooldown = 15 * time.Minute

func isRateLimit(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "rate limit") || strings.Contains(message, "rate-limit") ||
		strings.Contains(message, "too many requests") || strings.Contains(message, "http 429") ||
		strings.Contains(message, "status code 429") || strings.Contains(message, "status 429")
}

func (m *Manager) internalRateLimitKey(serverName string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, ss := range m.servers {
		if ss.cfg.Name == serverName && ss.cfg.Internal {
			// A changed endpoint or API key must not inherit the old cooldown.
			data, _ := json.Marshal([]any{ss.cfg.Name, ss.cfg.URL, ss.cfg.Headers, ss.cfg.OAuth})
			return fmt.Sprintf("%x", sha256.Sum256(data))
		}
	}
	return ""
}

func (m *Manager) checkInternalRateLimit(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	m.rateMu.Lock()
	defer m.rateMu.Unlock()
	m.loadRateLimits(ctx, false)
	if until := m.rateLimits[key]; time.Now().Before(until) {
		return fmt.Errorf("MCP adapter rate limit: requests suspended until %s", until.Format(time.RFC3339))
	}
	return nil
}

func (m *Manager) recordInternalRateLimit(ctx context.Context, key string) {
	if key == "" {
		return
	}
	m.rateMu.Lock()
	defer m.rateMu.Unlock()
	if m.rateLimits == nil {
		m.rateLimits = make(map[string]time.Time)
	}
	m.rateLimits[key] = time.Now().Add(internalRateLimitCooldown)
	// Preserve the backoff even if the originating prompt was interrupted.
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	m.loadRateLimits(persistCtx, true)
}

// Called with rateMu held. Merge under a file lock so search/fetch and separate
// runtimes share cooldowns without losing updates. Memory remains a fallback.
func (m *Manager) loadRateLimits(ctx context.Context, write bool) {
	path := m.options.InternalRateLimitPath
	if path == "" {
		return
	}
	err := func() error {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		lock := flock.New(path + ".lock")
		locked, err := lock.TryLockContext(ctx, 10*time.Millisecond)
		if err != nil {
			return err
		}
		if !locked {
			return fmt.Errorf("cooldown file lock unavailable")
		}
		defer lock.Unlock()
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		stored := make(map[string]time.Time)
		if len(data) > 0 {
			if err := json.Unmarshal(data, &stored); err != nil {
				return err
			}
		}
		if m.rateLimits == nil {
			m.rateLimits = make(map[string]time.Time)
		}
		for key, until := range stored {
			if until.After(m.rateLimits[key]) {
				m.rateLimits[key] = until
			}
		}
		for key, until := range m.rateLimits {
			if !time.Now().Before(until) {
				delete(m.rateLimits, key)
			}
		}
		if !write {
			return nil
		}
		data, err = json.Marshal(m.rateLimits)
		if err != nil {
			return err
		}
		file, err := os.CreateTemp(filepath.Dir(path), ".web-adapter-cooldowns-*")
		if err != nil {
			return err
		}
		defer os.Remove(file.Name())
		_, err = file.Write(data)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		return os.Rename(file.Name(), path)
	}()
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "MCP adapter cooldown persistence failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
	}
}
