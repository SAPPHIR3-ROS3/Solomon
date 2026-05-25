package commands

import (
	"context"
	"fmt"
	"strings"

	cursorint "github.com/SAPPHIR3-ROS3/Solomon/internal/integrations/cursor"
)

func SlashIntegrations(d Deps) error {
	ctx := d.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	st := cursorint.DefaultManager().ProxyStatus(ctx)
	var lines []string
	lines = append(lines, "Cursor API sidecar:")
	lines = append(lines, fmt.Sprintf("  url: %s", st.BaseURL))
	lines = append(lines, fmt.Sprintf("  health: %s", healthLabel(st.Healthy)))
	lines = append(lines, fmt.Sprintf("  managed by Solomon: %s", yesNo(st.Managed)))
	if st.InstallDir != "" {
		lines = append(lines, fmt.Sprintf("  install dir: %s", st.InstallDir))
	}
	if p := d.Provider(); p != nil && p.IsCursorAPI() {
		lines = append(lines, fmt.Sprintf("  current provider: %s (model %s)", p.Name, d.Model()))
	}
	PrintSystem(d.Out, strings.Join(lines, "\n"))
	return nil
}

func healthLabel(ok bool) string {
	if ok {
		return "ok"
	}
	return "not reachable"
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
