package skills

import (
	"sort"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

func InstalledSlashCommandNames(projHex, projRoot string) ([]string, error) {
	regPath, err := paths.SkillsRegistryPath()
	if err != nil {
		return nil, err
	}
	reg, err := LoadRegistry(regPath)
	if err != nil {
		return nil, err
	}
	refs := OrderedSkillRefs(reg, projHex, projRoot)
	bindings := AssignSkillSlashCommands(refs)
	seen := make(map[string]struct{}, len(bindings))
	out := make([]string, 0, len(bindings))
	for _, b := range bindings {
		s := b.Slash
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out, nil
}
