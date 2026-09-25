package test

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

func TestUpgradeFlow_execInstallRestartNoOpOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows exec install restart guard")
	}
	var scheduleCalls int
	restore := updater.SetScheduleInstallRestartHook(func(context.Context, string, io.Writer) error {
		scheduleCalls++
		return nil
	})
	defer restore()
	if err := updater.ExecInstallRestartForTest(context.Background(), "v2099.1.0"); err != nil {
		t.Fatal(err)
	}
	if scheduleCalls != 0 {
		t.Fatalf("ExecInstallRestart on windows must not schedule install, got %d calls", scheduleCalls)
	}
}

func TestUpgradeFlow_windowsInstallRestartScriptUsesUpgradeLock(t *testing.T) {
	t.Parallel()
	body, err := updater.WindowsInstallRestartScriptBodyForTest(1, "v2099.1.0", "", `C:\go\bin\solomon.exe`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "solomon-upgrade.lock") {
		t.Fatal("expected upgrade lock in install restart script")
	}
	if !strings.Contains(body, "solomon-dl-") {
		t.Fatal("expected unique download temp path in install restart script")
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

func TestUpgradeFlow_windowsInstallRestartScriptHandsOffManagedServer(t *testing.T) {
	t.Parallel()
	body, err := updater.WindowsInstallRestartScriptBodyForTest(1234, "v2026.925.0", t.TempDir(), `C:\Program Files\Solomon\solomon.exe`, []string{"."})
	if err != nil {
		t.Fatalf("build Windows install restart script: %v", err)
	}

	for _, want := range []string{
		"/_solomon/stop",
		"Start-SavedSolomonServer",
		"[System.IO.File]::Replace($staging, $Target, $null)",
		"cannot update while other Solomon instances are running",
		"started_at",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("generated script does not contain %q", want)
		}
	}

	powershell, err := exec.LookPath("pwsh")
	if err != nil {
		powershell, err = exec.LookPath("powershell")
	}
	if err != nil {
		t.Skip("PowerShell is not installed; skipping generated-script syntax check")
	}

	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "install-restart.ps1")
	if err := os.WriteFile(scriptPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write generated PowerShell script: %v", err)
	}
	parserPath := filepath.Join(tempDir, "parse.ps1")
	parserScript := `$tokens = $null
$parseErrors = $null
[System.Management.Automation.Language.Parser]::ParseFile($args[0], [ref]$tokens, [ref]$parseErrors) | Out-Null
if ($parseErrors.Count -gt 0) {
  $parseErrors | ForEach-Object { Write-Error $_.Message }
  exit 1
}`
	if err := os.WriteFile(parserPath, []byte(parserScript), 0o600); err != nil {
		t.Fatalf("write PowerShell parser helper: %v", err)
	}
	cmd := exec.Command(powershell, "-NoLogo", "-NoProfile", "-NonInteractive", "-File", parserPath, scriptPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("parse generated PowerShell script: %v\n%s", err, output)
	}
}
