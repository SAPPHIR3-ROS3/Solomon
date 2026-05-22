package commands

import (
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/skills"
)

func Remove(d Deps, parts []string) error {
	if len(parts) >= 1 {
		switch strings.ToLower(strings.TrimSpace(parts[0])) {
		case "rule":
			return RemoveRule(d, parts)
		case "projectrule":
			return RemoveProjectRule(d, parts)
		}
	}
	if d.ProjRoot == "" {
		return fmt.Errorf("remove: missing project root")
	}
	if d.ProjHex == "" {
		return fmt.Errorf("remove: missing project id")
	}
	opts := skills.RemoveOpts{
		Out:      d.Out,
		ProjHex:  d.ProjHex,
		ProjRoot: d.ProjRoot,
		Args:     parts,
	}
	return skills.RunRemove(opts)
}
