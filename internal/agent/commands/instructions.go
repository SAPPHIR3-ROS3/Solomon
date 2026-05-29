package commands

import (
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/instructions"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)
func Instructions(d Deps) error {
	path, err := paths.GlobalAgentsPath()
	if err != nil {
		return err
	}
	loader := instructions.NewLoader()
	loadedPath, content, ok := loader.LoadGlobal()
	if !ok {
		PrintSystemf(d.Out, "Global AGENTS: not found (%s)", path)
		return nil
	}
	st, err := os.Stat(loadedPath)
	if err != nil {
		return err
	}
	PrintSystemf(d.Out, "Global AGENTS: %s (%d bytes)\n\n%s", loadedPath, st.Size(), content)
	return nil
}
