package agentruntime

import (
	"context"

	cursoragent "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor/agent"
)

func (r *Runtime) ensureCursorSidecar(ctx context.Context) error {
	return cursoragent.EnsureSidecar(ctx, r.Cfg, r.Prov, r.ProjRoot, r.Out)
}
