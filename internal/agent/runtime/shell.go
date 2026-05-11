package agentruntime

import (
	"context"
	"os"
	"path/filepath"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/agent/tools"
)

func (r *Runtime) runUserShellLine(ctx context.Context, script string) error {
	wd := r.ProjRoot
	if p, err := filepath.Abs(r.ProjRoot); err == nil {
		wd = p
	}
	c := tools.NewShellCommand(ctx, wd, script)
	c.Stdout = r.Out
	c.Stderr = r.Out
	c.Stdin = os.Stdin
	return c.Run()
}
