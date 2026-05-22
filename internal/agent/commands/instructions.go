package commands

import (
	"fmt"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/instructions"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/paths"
)

func Instructions(d Deps) error {
	path, err := paths.GlobalAgentsPath()
	if err != nil {
		return err
	}
	loader := instructions.NewLoader()
	loadedPath, content, ok := loader.LoadGlobal()
	if !ok {
		fmt.Fprintf(d.Out, "Global AGENTS: not found (%s)\n", path)
		return nil
	}
	st, err := os.Stat(loadedPath)
	if err != nil {
		return err
	}
	fmt.Fprintf(d.Out, "Global AGENTS: %s (%d bytes)\n\n", loadedPath, st.Size())
	fmt.Fprintln(d.Out, content)
	return nil
}
