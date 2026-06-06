package shelllex

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	pathCmdMu     sync.Mutex
	pathCmdCache  map[string]struct{}
	pathCmdEnvKey string
)

func CommandKnown(name string) (found, isBuiltin bool) {
	name = normalizeCommandName(name)
	if name == "" {
		return false, false
	}
	if _, ok := ShellBuiltinsMap()[name]; ok {
		return true, true
	}
	if _, err := exec.LookPath(name); err == nil {
		return true, false
	}
	ensurePathCommandCache()
	_, found = pathCmdCache[name]
	return found, false
}

func normalizeCommandName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(strings.ToLower(name), ".exe")
	if i := strings.LastIndexAny(name, "/\\"); i >= 0 {
		name = name[i+1:]
	}
	return name
}

func ensurePathCommandCache() {
	pathEnv := os.Getenv("PATH")
	pathCmdMu.Lock()
	defer pathCmdMu.Unlock()
	if pathCmdCache != nil && pathCmdEnvKey == pathEnv {
		return
	}
	pathCmdEnvKey = pathEnv
	pathCmdCache = make(map[string]struct{})
	if pathEnv == "" {
		return
	}
	for _, dir := range filepath.SplitList(pathEnv) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			stem := executableStem(e.Name())
			if stem == "" {
				continue
			}
			pathCmdCache[strings.ToLower(stem)] = struct{}{}
		}
	}
}

func isPathExecutableFile(name string) bool {
	if name == "" || name[0] == '.' {
		return false
	}
	if runtime.GOOS != "windows" {
		return true
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return false
	}
	for _, pe := range patExts() {
		if ext == strings.ToLower(pe) {
			return true
		}
	}
	return false
}

func patExts() []string {
	raw := os.Getenv("PATHEXT")
	if raw == "" {
		return []string{".COM", ".EXE", ".BAT", ".CMD"}
	}
	var exts []string
	for _, p := range strings.Split(raw, ";") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p[0] != '.' {
			p = "." + p
		}
		exts = append(exts, p)
	}
	return exts
}

func executableStem(name string) string {
	if !isPathExecutableFile(name) {
		return ""
	}
	if runtime.GOOS != "windows" {
		return name
	}
	lower := strings.ToLower(name)
	for _, pe := range patExts() {
		peLower := strings.ToLower(pe)
		if strings.HasSuffix(lower, peLower) {
			return name[:len(name)-len(pe)]
		}
	}
	return name
}

func ShellBuiltinsMap() map[string]struct{} {
	out := make(map[string]struct{})
	shell := strings.ToLower(os.Getenv("SHELL"))
	comspec := strings.ToLower(os.Getenv("ComSpec"))
	switch {
	case strings.Contains(shell, "fish"):
		for _, n := range []string{"cd", "pwd", "export", "set"} {
			out[n] = struct{}{}
		}
	case strings.Contains(comspec, "cmd.exe") || comspec == "cmd":
		for _, n := range []string{"cd", "dir", "echo", "set"} {
			out[n] = struct{}{}
		}
	case strings.Contains(shell, "powershell") || strings.HasSuffix(shell, "pwsh"):
		for _, n := range []string{"cd", "dir", "ls", "pwd"} {
			out[n] = struct{}{}
		}
	default:
		for _, n := range []string{"alias", "bg", "cd", "echo", "export", "fg", "jobs", "pwd", "source", "type", "unset"} {
			out[n] = struct{}{}
		}
	}
	return out
}

func PathBinCandidates(prefix string) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for name := range ShellBuiltinsMap() {
		if matchBinPrefix(name, prefix) {
			add(name)
		}
	}
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return out
	}
	for _, dir := range filepath.SplitList(pathEnv) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			stem := executableStem(e.Name())
			if stem == "" {
				continue
			}
			if matchBinPrefix(stem, prefix) {
				add(stem)
			}
		}
	}
	return out
}

func matchBinPrefix(name, prefix string) bool {
	if prefix == "" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix))
}
