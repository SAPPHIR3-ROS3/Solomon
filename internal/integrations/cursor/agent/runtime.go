package agent

import (
	"context"
	"io"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	cursorint "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

type bootstrapOut struct {
	out io.Writer
}

func (b bootstrapOut) Print(msg string) {
	if b.out == nil {
		return
	}
	_, _ = io.WriteString(b.out, termcolor.SystemMessageText(msg)+"\n")
}

var _ cursorint.BootstrapIO = bootstrapOut{}

func BootstrapIO(out io.Writer) cursorint.BootstrapIO {
	if out == nil {
		return nil
	}
	return bootstrapOut{out: out}
}

func EnsureSidecar(ctx context.Context, cfg *config.Root, prov *config.Provider, projRoot string, out io.Writer) error {
	if cfg == nil || prov == nil || !prov.IsCursorAPI() {
		return nil
	}
	cwd := projRoot
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	return cursorint.WaitSidecarIfConfigured(ctx, cfg, cwd, BootstrapIO(out))
}
