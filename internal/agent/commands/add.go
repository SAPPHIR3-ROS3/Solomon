package commands

import (
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/skills"
)

func Add(d Deps, parts []string) error {
	if len(parts) >= 1 {
		switch strings.ToLower(strings.TrimSpace(parts[0])) {
		case "rule":
			return AddRule(d, parts[1:])
		case "projectrule":
			return AddProjectRule(d, parts[1:])
		}
	}
	if d.ProjRoot == "" {
		return fmt.Errorf("add: missing project root")
	}
	if d.ProjHex == "" {
		return fmt.Errorf("add: missing project id")
	}
	opts := skills.InstallOpts{
		Ctx:      d.Ctx,
		Out:      d.Out,
		In:       d.Stdin,
		ProjHex:  d.ProjHex,
		ProjRoot: d.ProjRoot,
		Args:     parts,
	}
	if err := skills.RunInstall(opts); err != nil {
		return err
	}
	PrintSystem(d.Out, "Skill installed and registry updated.")
	return nil
}
