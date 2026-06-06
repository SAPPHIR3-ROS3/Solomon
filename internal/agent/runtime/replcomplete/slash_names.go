package replcomplete

import (
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/skills"
)

func SlashCommandNames(env ReplCompleteEnv) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(name string) {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for _, n := range commands.SlashBuiltinNames() {
		add(n)
	}
	if env.ProjHex != "" || env.ProjRoot != "" {
		skillNames, err := skills.InstalledSlashCommandNames(env.ProjHex, env.ProjRoot)
		if err == nil {
			for _, n := range skillNames {
				add(n)
			}
		}
	}
	return out
}

func SlashCommandKnown(env ReplCompleteEnv, name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	for _, n := range SlashCommandNames(env) {
		if n == name {
			return true
		}
	}
	return false
}
