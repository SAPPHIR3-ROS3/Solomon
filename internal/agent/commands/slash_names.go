package commands

import (
	"sort"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

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
