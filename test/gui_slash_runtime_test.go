package test

import (
	"context"
	"io"
	"testing"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

func TestLocalCommandsSkipAgentStartup(t *testing.T) {
	logging.LogInit()
	cfg := &config.Root{}
	// A provider may be configured, but local commands must not authenticate or start it.
	prov := &config.Provider{}
	for _, line := range []string{"/fast", "/reasoning", "/help", "/version", "/name", "/terminal"} {
		rt := agentruntime.NewCommandRuntime(context.Background(), cfg, prov, "", "", &chatstore.Session{}, line)
		rt.Out = io.Discard
		if rt.StartedAgentResourcesForTest() {
			t.Fatalf("%s started agent resources", line)
		}
		if err := rt.WaitProviderReadyForTest(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := rt.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for _, line := range []string{"/docs question", "/exec prompt", "/summarize", "/research query", "/testweb", "/skill:example"} {
		if !agentruntime.CommandNeedsAgentRuntimeForTest(line, cfg) {
			t.Errorf("%s must retain agent dependencies", line)
		}
	}
}
