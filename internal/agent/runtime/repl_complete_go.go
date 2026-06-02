package agentruntime

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	goSubcmdCacheMu sync.Mutex
	goSubcmdCache   []string
	goSubcmdCacheKey string
)

var goHelpCommandLine = regexp.MustCompile(`^\t([a-z][a-z0-9]*)\s`)

func goSubcommandCandidates() []string {
	if list, ok := cachedGoSubcommands(); ok {
		return list
	}
	list, key := loadGoSubcommandsFromToolchain()
	goSubcmdCacheMu.Lock()
	goSubcmdCache = list
	goSubcmdCacheKey = key
	goSubcmdCacheMu.Unlock()
	return list
}

func cachedGoSubcommands() ([]string, bool) {
	goSubcmdCacheMu.Lock()
	defer goSubcmdCacheMu.Unlock()
	if goSubcmdCache == nil && goSubcmdCacheKey == "" {
		return nil, false
	}
	key, err := goVersionKey()
	if err != nil || key != goSubcmdCacheKey {
		return nil, false
	}
	out := make([]string, len(goSubcmdCache))
	copy(out, goSubcmdCache)
	return out, true
}

func goVersionKey() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "version")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func loadGoSubcommandsFromToolchain() ([]string, string) {
	key, err := goVersionKey()
	if err != nil {
		return nil, ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "help")
	out, err := cmd.Output()
	if err != nil {
		return nil, key
	}
	seen := make(map[string]struct{})
	var list []string
	for _, line := range strings.Split(string(out), "\n") {
		m := goHelpCommandLine.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		name := m[1]
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		list = append(list, name)
	}
	return list, key
}

func ReplCompleteResetGoCacheForTest() {
	goSubcmdCacheMu.Lock()
	goSubcmdCache = nil
	goSubcmdCacheKey = ""
	goSubcmdCacheMu.Unlock()
}
