package programming

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

type Server struct {
	Command   []string `json:"command"`
	Available bool     `json:"available"`
	Source    string   `json:"source"`
	Reason    string   `json:"reason,omitempty"`
}

func resolveCommand(candidate []string, source string) (Server, bool) {
	if len(candidate) == 0 {
		return Server{}, false
	}
	path, err := exec.LookPath(candidate[0])
	if err != nil {
		return Server{}, false
	}
	if source == "detected" && candidate[0] == "dart" {
		sdk := filepath.Join(filepath.Dir(path), "cache", "dart-sdk", "bin", "dart")
		if info, err := os.Stat(sdk); err == nil && !info.IsDir() {
			path = sdk
		} else if _, err := os.Stat(filepath.Join(filepath.Dir(path), "flutter")); err == nil {
			return Server{}, false
		}
	}
	return Server{Command: append([]string{path}, candidate[1:]...), Available: true, Source: source}, true
}

func InstalledServers(id string, language config.ProgrammingLanguage) []Server {
	servers := make([]Server, 0)
	seen := map[string]bool{}
	add := func(command []string, source string) {
		if server, ok := resolveCommand(command, source); ok {
			key, _ := json.Marshal(server.Command)
			if !seen[string(key)] {
				servers = append(servers, server)
				seen[string(key)] = true
			}
		}
	}
	for _, candidate := range automaticServers[strings.TrimPrefix(id, "worktree-")] {
		add(candidate, "detected")
	}
	add(language.Command, "custom")
	return servers
}

func Resolve(id string, language config.ProgrammingLanguage) Server {
	if language.Mode == "fallback" {
		return Server{Source: "internal", Reason: "Internal fallback selected"}
	}
	if language.Mode == "custom" {
		if server, ok := resolveCommand(language.Command, "custom"); ok {
			return server
		}
		return Server{Source: "internal", Reason: "The configured language server executable was not found"}
	}
	for _, candidate := range automaticServers[strings.TrimPrefix(id, "worktree-")] {
		if server, ok := resolveCommand(candidate, "detected"); ok {
			return server
		}
	}
	return Server{Source: "internal", Reason: "No compatible installed language server detected"}
}
