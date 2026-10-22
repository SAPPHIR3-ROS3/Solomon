package tools

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func editInvocationPaths(env *Env, raw json.RawMessage) []string {
	var args editArgs
	if json.Unmarshal(raw, &args) != nil || env == nil {
		return nil
	}
	paths := []string{resolveProjectPath(env.ProjRoot, args.Path)}
	if args.RenameTo != "" {
		paths = append(paths, resolveProjectPath(env.ProjRoot, args.RenameTo))
	}

	return paths
}
func shellInvocationPaths(env *Env, raw json.RawMessage) []string {
	var args struct {
		Command string `json:"command"`
	}
	if json.Unmarshal(raw, &args) != nil || env == nil {
		return nil
	}
	return ShellFileChangePaths(env.ProjRoot, args.Command)
}

// Collect literal paths without evaluating substitutions or running the command
// a second time. This also finds paths in quoted Python/shell snippets. Unknown
// dynamically-computed targets fall back to the normal project snapshot.
var shellPathRE = regexp.MustCompile(`(?:\$\{[A-Za-z_][A-Za-z_0-9]*\}|\$[A-Za-z_][A-Za-z_0-9]*|~|\.{1,2}|[A-Za-z_][A-Za-z_0-9.-]*)?/[A-Za-z0-9_./@+%:=\-{}$~*?]+`)
var shellRedirectRE = regexp.MustCompile(`(?:>{1,2}|<)\s*["']?([^\s"'<>;&|]+)`)

var quotedShellPathRE = regexp.MustCompile(`["']((?:/|~/|\$[A-Za-z_][A-Za-z_0-9]*/|\$\{[A-Za-z_][A-Za-z_0-9]*\}/|\.{1,2}/)[^"'\n]+)["']`)

var xdgShellDirectoryRE = regexp.MustCompile(`\bxdg-user-dir\s+["']?(DESKTOP|DOWNLOAD|DOCUMENTS|MUSIC|PICTURES|PUBLICSHARE|TEMPLATES|VIDEOS)\b`)
var xdgShellAssignmentRE = regexp.MustCompile(`([A-Za-z_][A-Za-z_0-9]*)=["']?\$\(\s*xdg-user-dir\s+["']?(DESKTOP|DOWNLOAD|DOCUMENTS|MUSIC|PICTURES|PUBLICSHARE|TEMPLATES|VIDEOS)\b`)
var literalShellAssignmentRE = regexp.MustCompile(`(?:^|[;\n \t])([A-Za-z_][A-Za-z_0-9]*)=(?:"([^"\n]*)"|'([^'\n]*)'|([^\s;]+))`)

func ShellFileChangePaths(root, command string) []string {
	seen := map[string]bool{}
	var paths []string
	variables := map[string]string{}
	for _, match := range xdgShellAssignmentRE.FindAllStringSubmatch(command, -1) {
		if dir := xdgShellDirectory(match[2]); dir != "" {
			variables[match[1]] = dir
		}
	}
	expand := func(word string) (string, bool) {
		known := true
		value := os.Expand(word, func(key string) string {
			if v, ok := variables[key]; ok {
				return v
			}
			v, ok := os.LookupEnv(key)
			if !ok {
				known = false
			}
			return v
		})
		return value, known
	}
	for _, match := range literalShellAssignmentRE.FindAllStringSubmatch(command, -1) {
		value := match[2]
		if value == "" {
			value = match[3]
		}
		if value == "" {
			value = match[4]
		}
		if strings.Contains(value, "$(") || strings.Contains(value, "`") {
			continue
		}
		if value, ok := expand(value); ok {
			variables[match[1]] = value
		}
	}
	add := func(word string) {
		word = strings.Trim(word, "\\\"'")
		if strings.ContainsAny(word, "`\x00\n") || strings.Contains(word, "$(") {
			return
		}
		if strings.HasPrefix(word, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				word = filepath.Join(home, word[2:])
			}
		}
		var known bool
		word, known = expand(word)
		if !known {
			return
		}
		if word == "" {
			return
		}
		if !filepath.IsAbs(word) {
			word = filepath.Join(root, word)
		}
		word = filepath.Clean(word)
		if word == string(filepath.Separator) || word == filepath.Clean(root) {
			return
		}
		matches, err := filepath.Glob(word)
		if err == nil && len(matches) > 0 {
			for _, match := range matches {
				if !seen[match] {
					seen[match] = true
					paths = append(paths, match)
				}
			}
			return
		}
		if !seen[word] {
			seen[word] = true
			paths = append(paths, word)
		}
	}
	// Snapshot the resolved directory, not just the initially chosen filename:
	// shell loops may select test-1.txt or another target only during execution.
	for _, match := range xdgShellDirectoryRE.FindAllStringSubmatch(command, -1) {
		dir := xdgShellDirectory(match[1])
		if dir == "" {
			continue
		}
		if filepath.Clean(dir) == filepath.Clean(root) {
			if !seen[root] {
				seen[root] = true
				paths = append(paths, root)
			}
		} else {
			add(dir)
		}
	}
	for _, match := range quotedShellPathRE.FindAllStringSubmatch(command, -1) {
		add(match[1])
	}
	for _, word := range shellPathRE.FindAllString(command, -1) {
		add(word)
	}
	for _, match := range shellRedirectRE.FindAllStringSubmatch(command, -1) {
		add(match[1])
	}
	if len(paths) > 0 {
		if err := exec.Command("git", "-C", root, "rev-parse", "--show-toplevel").Run(); err == nil {
			paths = append(paths, root)
		}
	}
	return paths
}

// Read XDG configuration as data. Never execute the shell command (or source
// user-dirs.dirs) to resolve a path for tracking.
func xdgShellDirectory(kind string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	defaults := map[string]string{"DESKTOP": "Desktop", "DOWNLOAD": "Downloads", "DOCUMENTS": "Documents", "MUSIC": "Music", "PICTURES": "Pictures", "PUBLICSHARE": "Public", "TEMPLATES": "Templates", "VIDEOS": "Videos"}
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		configDir = filepath.Join(home, ".config")
	}
	data, _ := os.ReadFile(filepath.Join(configDir, "user-dirs.dirs"))
	key := "XDG_" + kind + "_DIR"
	for _, line := range strings.Split(string(data), "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || strings.TrimSpace(name) != key {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), "\"'")
		if strings.Contains(value, "$(") || strings.Contains(value, "`") {
			break
		}
		value = os.Expand(value, func(name string) string {
			if name == "HOME" {
				return home
			}
			return os.Getenv(name)
		})
		if filepath.IsAbs(value) {
			return filepath.Clean(value)
		}
	}
	if folder, ok := defaults[kind]; ok {
		return filepath.Join(home, folder)
	}
	return ""
}
