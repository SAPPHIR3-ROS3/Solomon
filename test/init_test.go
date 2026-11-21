package test

import (
	"os"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	sandboxworker "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/worker"
)

func setSolomonHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("SOLOMON_HOME", home)
	t.Cleanup(func() {
		if err := database.ReleaseDataPool(home); err != nil {
			t.Error(err)
		}
		if err := logging.ReleaseLogDatabase(home); err != nil {
			t.Error(err)
		}
	})
}

func TestMain(m *testing.M) {
	if len(os.Args) >= 2 && os.Args[1] == "sandbox-worker" {
		sandboxworker.Main()
		return
	}
	logging.LogInit(logging.INFO_LOG_LEVEL)
	os.Exit(m.Run())
}
