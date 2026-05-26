package cursor

import (
	"context"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
)

func EnsureSidecarIfConfigured(ctx context.Context, cfg *config.Root, cwd string, out BootstrapIO) error {
	return WaitSidecarIfConfigured(ctx, cfg, cwd, out)
}
