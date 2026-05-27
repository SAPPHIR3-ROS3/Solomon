package agentruntime

import (
	"context"
	"io"
	"os"

	cursorint "github.com/SAPPHIR3-ROS3/Solomon/internal/integrations/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"
)

type runtimeBootstrapOut struct {
	out io.Writer
}

func (r runtimeBootstrapOut) Print(msg string) {
	if r.out == nil {
		return
	}
	_, _ = io.WriteString(r.out, termcolor.SystemMessageText(msg)+"\n")
}

var _ cursorint.BootstrapIO = runtimeBootstrapOut{}

func (r *Runtime) ensureCursorSidecar(ctx context.Context) error {
	if r == nil || r.Cfg == nil || r.Prov == nil || !r.Prov.IsCursorAPI() {
		return nil
	}
	cwd := r.ProjRoot
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	return cursorint.WaitSidecarIfConfigured(ctx, r.Cfg, cwd, nil)
}
