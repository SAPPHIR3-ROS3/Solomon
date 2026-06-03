package test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func TestRunSystemInstallSchedulesRestart(t *testing.T) {
	t.Parallel()
	restore := updater.SetScheduleInstallRestartHook(func(context.Context, string, io.Writer) error {
		return nil
	})
	defer restore()
	err := updater.RunSystemInstall(context.Background(), "v2099.1.0", io.Discard)
	if !errors.Is(err, updater.ErrRestartScheduled) {
		t.Fatalf("got %v", err)
	}
}
