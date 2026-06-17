package agentruntime

import (
	"context"
	"io"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/turnloop"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/instructions"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooloutput"
)

var execToolHook func(context.Context, tooling.Invocation) (any, error)

func NewTestRuntime(cfg *config.Root, prov *config.Provider, projHex, projRoot string, sess *chatstore.Session, out io.Writer) *Runtime {
	if out == nil {
		out = io.Discard
	}
	return &Runtime{
		Model:                     cfg.Current.Model,
		Cfg:                       cfg,
		Prov:                      prov,
		ProjHex:                   projHex,
		ProjRoot:                  projRoot,
		Mode:                      "agent",
		Session:                   sess,
		CompactionThresholdTokens: config.EffectiveCompactionThresholdTokens(cfg),
		Out:                       out,
		ToolOut:                   tooloutput.NewService(projHex, tooloutput.LimitsFromConfig(cfg)),
		Instructions:              instructions.NewLoader(),
	}
}

func (r *Runtime) RunAgentTurnsForTest(ctx context.Context) error {
	return r.runAgentTurns(ctx)
}

func StopAgentGenerationForTest() {
	turnloop.StopForTest(errUserStopGeneration)
}

func SetExecToolHookForTest(fn func(context.Context, tooling.Invocation) (any, error)) func() {
	prev := execToolHook
	execToolHook = fn
	return func() { execToolHook = prev }
}
