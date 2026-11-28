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
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/weblimit"
	"github.com/gofrs/flock"
)

func (m *Manager) internalRateLimitKey(serverName string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, ss := range m.servers {
		if ss.cfg.Name == serverName && ss.cfg.Internal {
			return internalRateLimitKey(ss.cfg)
		}
	}
	return ""
}

func internalRateLimitKey(cfg ServerConfig) string {
	if !cfg.Internal {
		return ""
	}
	data, _ := json.Marshal([]any{cfg.Name, cfg.URL, cfg.Headers, cfg.OAuth})
	return fmt.Sprintf("mcp:%x", sha256.Sum256(data))
}

func (m *Manager) CheckInternalRateLimit(ctx context.Context, serverName string) error {
	return m.checkInternalRateLimit(ctx, m.internalRateLimitKey(serverName))
}

func (m *Manager) SetRateLimitChat(chat string) {
	m.rateMu.Lock()
	defer m.rateMu.Unlock()
	m.rateLimitChat = chat
}

func (m *Manager) chatRateLimitKey(key string) string {
	if m.rateLimitChat == "" {
		return "local:" + key
	}
	return fmt.Sprintf("chat:%x:%s", sha256.Sum256([]byte(m.rateLimitChat)), key)
}

func (m *Manager) checkInternalRateLimit(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	m.rateMu.Lock()
	defer m.rateMu.Unlock()
	m.loadRateLimits(ctx, false)
	if until := m.rateLimits[m.chatRateLimitKey(key)]; until.Equal(weblimit.UnknownReset) {
		return &weblimit.Suspended{Until: until}
	}
	if until := m.rateLimits[key]; time.Now().Before(until) {
		return &weblimit.Suspended{Until: until}
	}
	return nil
}

func (m *Manager) recordInternalRateLimit(ctx context.Context, key string, until time.Time) {
	if key == "" {
		return
	}
	m.rateMu.Lock()
	defer m.rateMu.Unlock()
	if m.rateLimits == nil {
		m.rateLimits = make(map[string]time.Time)
	}
	if until.Equal(weblimit.UnknownReset) {
		key = m.chatRateLimitKey(key)
	}
	if until.After(m.rateLimits[key]) {
		m.rateLimits[key] = until
	}
	if strings.HasPrefix(key, "local:") {
		return
	}
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
			if len(key) == sha256.Size*2 || strings.HasPrefix(key, "local:") || (until.Equal(weblimit.UnknownReset) && !strings.HasPrefix(key, "chat:")) {
				continue
			}
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
		clear(stored)
		for key, until := range m.rateLimits {
			if !strings.HasPrefix(key, "local:") {
				stored[key] = until
			}
		}
		data, err = json.Marshal(stored)
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

func (m *Manager) CheckBrowserRateLimit(ctx context.Context, origin string) error {
	return m.checkInternalRateLimit(ctx, "browser:"+origin)
}

func (m *Manager) RecordBrowserRateLimit(ctx context.Context, origin string, until time.Time) {
	m.recordInternalRateLimit(ctx, "browser:"+origin, until)
}
