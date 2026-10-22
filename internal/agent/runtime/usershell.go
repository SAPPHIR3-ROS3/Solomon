package agentruntime

import (
	"context"
	"os"
	"path/filepath"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/multiline"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl/shellhist"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func (r *Runtime) releaseTTYForSubprocess() func() {
	if r.RL == nil {
		return func() {}
	}
	r.RL.Clean()
	_ = r.RL.Terminal.ExitRawMode()
	multiline.WriteTerminalModeSequences(multiline.BracketedPasteDisable + multiline.MouseReportDisable)
	restoreConsole := multiline.PrepareConsoleInput()
	return func() {
		restoreConsole()
		multiline.WriteTerminalModeSequences(multiline.BracketedPasteEnable)
		multiline.EnsureCookedTTY()
		r.RL.Refresh()
	}
}

func (r *Runtime) runUserShellLine(ctx context.Context, script string) error {
	wd := r.ProjRoot
	if p, err := filepath.Abs(r.ProjRoot); err == nil {
		wd = p
	}
	c := tools.NewShellCommand(ctx, wd, script)
	c.Stdout = r.Out
	c.Stderr = r.Out
	c.Stdin = os.Stdin
	release := r.releaseTTYForSubprocess()
	defer release()
	_, err := tools.WithFileChangeTrackingPaths(ctx, r.toolEnv(tooling.Invocation{}), tools.ShellFileChangePaths(r.ProjRoot, script), func() (any, error) { return nil, c.Run() })
	if err != nil {
		return err
	}
	if err := r.persistSession(); err != nil {
		return err
	}
	_ = shellhist.Append(script)
	return nil
}
