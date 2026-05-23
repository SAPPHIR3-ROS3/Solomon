package commands

import (
	"sort"
	"strings"
)

func SlashBuiltinNames() []string {
	tab := getSlashBuiltins()
	seen := make(map[string]struct{})
	var out []string
	for i := range tab {
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
