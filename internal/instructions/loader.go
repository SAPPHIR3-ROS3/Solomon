package instructions

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

const DefaultMaxFileBytes = 32 * 1024

type cachedEntry struct {
	modTime int64
	content string
}

type Loader struct {
	mu           sync.Mutex
	cache        map[string]cachedEntry
	MaxFileBytes int64
}

func NewLoader() *Loader {
	return &Loader{
		cache:        make(map[string]cachedEntry),
		MaxFileBytes: DefaultMaxFileBytes,
	}
}

func (l *Loader) maxBytes() int64 {
	if l == nil || l.MaxFileBytes <= 0 {
		return DefaultMaxFileBytes
	}
	return l.MaxFileBytes
}

func (l *Loader) readCached(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	st, err := os.Stat(path)
	if err != nil {
		l.dropCache(path)
		return "", false
	}
	mod := st.ModTime().UnixNano()
	l.mu.Lock()
	defer l.mu.Unlock()
	if e, ok := l.cache[path]; ok && e.modTime == mod {
		return e.content, true
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		l.dropCache(path)
		return "", false
	}
	content, truncated := truncateContent(string(raw), l.maxBytes())
	if truncated {
		logging.Log(logging.WARNING_LOG_LEVEL, "agents file truncated for system prompt", logging.LogOptions{Params: map[string]any{"path": path, "max_bytes": l.maxBytes()}})
	}
	l.cache[path] = cachedEntry{modTime: mod, content: content}
	return content, true
}

func (l *Loader) dropCache(path string) {
	l.mu.Lock()
	delete(l.cache, path)
	l.mu.Unlock()
}

func truncateContent(s string, max int64) (string, bool) {
	if max <= 0 || int64(len(s)) <= max {
		return s, false
	}
	omitted := int64(len(s)) - max
	footer := fmt.Sprintf("\n\n[truncated: %d bytes omitted — edit file to reduce]", omitted)
	cut := max - int64(len(footer))
	if cut < 0 {
		cut = 0
	}
	return s[:cut] + footer, true
}

func (l *Loader) LoadGlobal() (path, content string, ok bool) {
	p, err := paths.GlobalAgentsPath()
	if err != nil {
		return "", "", false
	}
	if c, ok := l.readCached(p); ok {
		return p, c, true
	}
	return "", "", false
}

func (l *Loader) LoadRepoRoot(projRoot string) (path, content string, ok bool) {
	if projRoot == "" {
		return "", "", false
	}
	if p, ok := FindAgentsFile(projRoot); ok {
		if c, ok := l.readCached(p); ok {
			return p, c, true
		}
	}
	return "", "", false
}

func (l *Loader) LoadRepoDir(projRoot, relDir string) (path, content string, ok bool) {
	if projRoot == "" || relDir == "" || relDir == "." || relDir == "/" {
		return "", "", false
	}
	abs := filepathJoin(projRoot, relDir)
	if p, ok := FindAgentsFile(abs); ok {
		if c, ok := l.readCached(p); ok {
			return p, c, true
		}
	}
	return "", "", false
}

func filepathJoin(root, rel string) string {
	return filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))
}
