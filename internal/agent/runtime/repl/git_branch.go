package repl

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type gitBranchCacheEntry struct {
	headMod int64
	branch  string
}

var gitBranchCache struct {
	mu   sync.Mutex
	byDir map[string]gitBranchCacheEntry
}

func init() {
	gitBranchCache.byDir = map[string]gitBranchCacheEntry{}
}

func cachedGitBranch(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	headPath := filepath.Join(abs, ".git", "HEAD")
	fi, err := os.Stat(headPath)
	if err != nil {
		return ""
	}
	mod := fi.ModTime().UnixNano()
	gitBranchCache.mu.Lock()
	if e, ok := gitBranchCache.byDir[abs]; ok && e.headMod == mod {
		br := e.branch
		gitBranchCache.mu.Unlock()
		return br
	}
	gitBranchCache.mu.Unlock()

	br := gitBranchQuick(abs)
	gitBranchCache.mu.Lock()
	gitBranchCache.byDir[abs] = gitBranchCacheEntry{headMod: mod, branch: br}
	gitBranchCache.mu.Unlock()
	return br
}

func gitBranchQuick(dir string) string {
	c := exec.Command("git", "-C", dir, "branch", "--show-current")
	out, err := c.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
