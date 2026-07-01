package test

import (
	"context"
	"errors"
	"io"
	"regexp"
	"runtime"
	"strings"
	"testing"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func TestUpgradeFlow_exitPathMatchesOSPolicy(t *testing.T) {
	t.Parallel()
	if agentruntime.UsesExecInstallRestartAfterSystemInstallForTest() != updater.UsesExecInstallRestartAfterSystemInstallForTest() {
		t.Fatal("upgrade exit policy mismatch between runtime and updater")
	}
}

func TestUpgradeFlow_runSystemInstallSchedulesBackgroundOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows background install restart")
	}
	var scheduleCalls int
	restore := updater.SetScheduleInstallRestartHook(func(context.Context, string, io.Writer) error {
		scheduleCalls++
		return nil
	})
	defer restore()
	err := updater.RunSystemInstall(context.Background(), "v2099.1.0", io.Discard)
	if !errors.Is(err, updater.ErrRestartScheduled) {
		t.Fatalf("got %v", err)
	}
	if scheduleCalls != 1 {
		t.Fatalf("expected one background install schedule, got %d", scheduleCalls)
	}
}

func TestUpgradeFlow_runSystemInstallDefersExecOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix exec install restart")
	}
	var scheduleCalls int
	restore := updater.SetScheduleInstallRestartHook(func(context.Context, string, io.Writer) error {
		scheduleCalls++
		return nil
	})
	defer restore()
	err := updater.RunSystemInstall(context.Background(), "v2099.1.0", io.Discard)
	if !errors.Is(err, updater.ErrRestartScheduled) {
		t.Fatalf("got %v", err)
	}
	if scheduleCalls != 0 {
		t.Fatalf("unix upgrade must defer to ExecInstallRestart, schedule calls=%d", scheduleCalls)
	}
}

func TestUpgradeFlow_exitDoesNotDoubleScheduleOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows duplicate install guard")
	}
	var scheduleCalls, execCalls int
	restoreSchedule := updater.SetScheduleInstallRestartHook(func(context.Context, string, io.Writer) error {
		scheduleCalls++
		return nil
	})
	defer restoreSchedule()
	restoreExec := updater.SetExecInstallRestartHook(func(context.Context, string) error {
		execCalls++
		return nil
	})
	defer restoreExec()

	err := updater.RunSystemInstall(context.Background(), "v2099.1.0", io.Discard)
	if !errors.Is(err, updater.ErrRestartScheduled) {
		t.Fatalf("got %v", err)
	}
	if err := updater.SimulateUpgradeExitRestartForTest("v2099.1.0"); err != nil {
		t.Fatal(err)
	}
	if scheduleCalls != 1 {
		t.Fatalf("expected one background schedule, got %d", scheduleCalls)
	}
	if execCalls != 0 {
		t.Fatalf("windows exit must not call ExecInstallRestart, got %d calls", execCalls)
	}
}

func TestUpgradeFlow_exitUsesExecInstallRestartOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix exec install restart")
	}
	var execCalls int
	restoreExec := updater.SetExecInstallRestartHook(func(context.Context, string) error {
		execCalls++
		return nil
	})
	defer restoreExec()

	err := updater.RunSystemInstall(context.Background(), "v2099.1.0", io.Discard)
	if !errors.Is(err, updater.ErrRestartScheduled) {
		t.Fatalf("got %v", err)
	}
	if err := updater.SimulateUpgradeExitRestartForTest("v2099.1.0"); err != nil {
		t.Fatal(err)
	}
	if execCalls != 1 {
		t.Fatalf("unix exit must call ExecInstallRestart once, got %d", execCalls)
	}
}

func TestUpgradeFlow_windowsInstallRestartScriptRequiresExe(t *testing.T) {
	t.Parallel()
	_, err := updater.WindowsInstallRestartScriptBodyForTest(1, "v2099.1.0", "", "", nil)
	if err == nil || !strings.Contains(err.Error(), "empty executable path") {
		t.Fatalf("expected empty executable error, got %v", err)
	}
}

func TestUpgradeFlow_windowsInstallRestartScriptSetsRestartExe(t *testing.T) {
	t.Parallel()
	exe := `C:\Users\patri\go\bin\solomon.exe`
	body, err := updater.WindowsInstallRestartScriptBodyForTest(42, "v2099.1.0", `D:\Projects\Golang\Solomon`, exe, []string{"--help"})
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`\$RestartExe = '([^']*)'`)
	m := re.FindStringSubmatch(body)
	if len(m) != 2 {
		t.Fatalf("missing RestartExe assignment in script:\n%s", body)
	}
	if m[1] != exe {
		t.Fatalf("RestartExe = %q, want %q", m[1], exe)
	}
	if !strings.Contains(body, "& $RestartExe @RestartArgs") {
		t.Fatal("expected restart line with args")
	}
}
