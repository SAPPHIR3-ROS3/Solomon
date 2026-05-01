package commands

import (
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/skills"
)

func Add(d Deps, parts []string) error {
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
	fmt.Fprintln(d.Out, "Skill installed and registry updated.")
	return nil
}
