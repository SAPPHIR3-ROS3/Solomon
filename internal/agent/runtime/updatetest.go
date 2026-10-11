package agentruntime

import (
	"context"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func UsesExecInstallRestartAfterSystemInstallForTest() bool {
	return updater.UsesExecInstallRestartAfterSystemInstall()
}

func (r *Runtime) StartedAgentResourcesForTest() bool {
	return !r.commandOnly || r.providerReady != nil || r.Backend != nil || r.MCP != nil || r.sandboxRetained
}

func (r *Runtime) WaitProviderReadyForTest(ctx context.Context) error {
	return r.waitProviderReady(ctx)
}

func CommandNeedsAgentRuntimeForTest(line string, cfg *config.Root) bool {
	return commandNeedsAgentRuntime(line, cfg)
}
