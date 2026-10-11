package agentruntime

import (
	"context"
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/slash"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

// RunGUISlash uses the REPL dispatcher without reading from the daemon's stdin
// or placing skill prompts in an invisible terminal input buffer.
func (r *Runtime) RunGUISlash(ctx context.Context, line string) error {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}
	name := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	if !commands.GUISlashAvailable(name) {
		return fmt.Errorf("/%s requires the terminal; use the corresponding GUI control", name)
	}
	if err := r.waitProviderReady(ctx); err != nil {
		return err
	}
	d := r.slashDeps(ctx)
	d.Stdin = strings.NewReader("")
	d.ReadLine = func(prompt string) (string, error) {
		return "", fmt.Errorf("this command requires interactive input; use the terminal or GUI settings")
	}
	d.PrefillInput = nil
	d.PrintWelcomeBanner = nil
	return slash.Dispatch(d, line)
}

// Local commands use the daemon dispatcher without starting agent resources.
func NewCommandRuntime(ctx context.Context, cfg *config.Root, prov *config.Provider, projectID, root string, sess *chatstore.Session, line string) *Runtime {
	if commandNeedsAgentRuntime(line, cfg) {
		rt := NewRuntime(nil, cfg, prov, projectID, root, sess)
		rt.InitMCP(ctx)
		return rt
	}
	rt := newRuntimeState(nil, cfg, prov, projectID, root, sess)
	rt.commandOnly = true
	return rt
}

func commandNeedsAgentRuntime(line string, cfg *config.Root) bool {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return false
	}
	name := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	if !commands.GUISlashAvailable(name) {
		return false
	}
	switch name {
	case "docs", "exec", "summarize", "compact", "research", "subagent", "testweb":
		return true
	}
	for _, builtin := range commands.SlashBuiltinNames(cfg) {
		if name == builtin {
			return false
		}
	}
	// Installed skills can submit agent prompts, so keep the full runtime for them.
	return true
}
