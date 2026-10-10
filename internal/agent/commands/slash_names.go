package commands

import (
	"sort"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

// GUISlashAvailable excludes commands whose lifecycle belongs to the terminal.
// Interactive arguments are rejected by the GUI's noninteractive input adapter.
func GUISlashAvailable(name string) bool {
	switch name {
	case "clear", "terminal", "models", "connect", "new", "temp", "resume", "exit", "quit", "btw", "onboard", "upgrade", "rewind":
		return false
	default:
		return true
	}
}

func SlashBuiltinDescriptions(cfg *config.Root) map[string]string {
	out := make(map[string]string)
	for _, b := range getSlashBuiltins() {
		if slashVisible(&b, cfg) {
			for _, name := range b.keys {
				out[name] = b.detail
			}
		}
	}
	return out
}

func SlashBuiltinNames(cfg *config.Root) []string {
	tab := getSlashBuiltins()
	seen := make(map[string]struct{})
	var out []string
	for i := range tab {
		if !slashVisible(&tab[i], cfg) {
			continue
		}
		for _, k := range tab[i].keys {
			k = strings.ToLower(strings.TrimSpace(k))
			if k == "" {
				continue
			}
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
