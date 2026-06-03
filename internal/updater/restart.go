package updater

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var ErrRestartScheduled = errors.New("restart scheduled after update")

var scheduleInstallRestart = defaultScheduleInstallRestart

func SetScheduleInstallRestartHook(fn func(context.Context, string, io.Writer) error) func() {
	prev := scheduleInstallRestart
	if fn == nil {
		scheduleInstallRestart = defaultScheduleInstallRestart
	} else {
		scheduleInstallRestart = fn
	}
	return func() { scheduleInstallRestart = prev }
}

func defaultScheduleInstallRestart(ctx context.Context, tag string, progress io.Writer) error {
	exe, err := restartExecutable()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	pid := os.Getpid()
	args := append([]string(nil), os.Args[1:]...)
	switch runtime.GOOS {
	case "linux", "darwin":
		return launchUnixInstallRestart(ctx, pid, tag, cwd, exe, args, progress)
	case "windows":
		return launchWindowsInstallRestart(ctx, pid, tag, cwd, exe, args, progress)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func restartExecutable() (string, error) {
	if p, err := installTargetPath(); err == nil {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return os.Executable()
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func launchUnixInstallRestart(ctx context.Context, pid int, tag, cwd, exe string, args []string, progress io.Writer) error {
	scriptPath, err := writeUnixInstallRestartScript(pid, tag, cwd, exe, args)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "bash", scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		_ = os.Remove(scriptPath)
		return err
	}
	go func() { _ = cmd.Wait(); _ = os.Remove(scriptPath) }()
	if progress != nil {
		fmt.Fprintln(progress, "Update will install after Solomon exits, then restart in this terminal.")
	}
	return nil
}

func writeUnixInstallRestartScript(pid int, tag, cwd, exe string, args []string) (string, error) {
	var b strings.Builder
	b.WriteString("#!/usr/bin/env bash\nset -euo pipefail\n")
	fmt.Fprintf(&b, "PARENT_PID=%d\n", pid)
	fmt.Fprintf(&b, "TAG=%s\n", shellQuote(tag))
	fmt.Fprintf(&b, "CWD=%s\n", shellQuote(cwd))
	fmt.Fprintf(&b, "RESTART_EXE=%s\n", shellQuote(exe))
	fmt.Fprintf(&b, "INSTALL_SCRIPT=%s\n", shellQuote(installScriptRawURL))
	b.WriteString("while kill -0 \"$PARENT_PID\" 2>/dev/null; do sleep 0.25; done\n")
	b.WriteString("if [[ -n \"$CWD\" ]]; then cd \"$CWD\"; fi\n")
	b.WriteString("export SOLOMON_VERSION=\"$TAG\"\n")
	b.WriteString("curl -fsSL \"$INSTALL_SCRIPT\" | bash\n")
	b.WriteString("RESTART_ARGS=(")
	for i, a := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(shellQuote(a))
	}
	b.WriteString(")\nexec \"$RESTART_EXE\" \"${RESTART_ARGS[@]}\"\n")
	return writeExecutableScript(b.String())
}

func launchWindowsInstallRestart(ctx context.Context, pid int, tag, cwd, exe string, args []string, progress io.Writer) error {
	scriptPath, err := writeWindowsInstallRestartScript(pid, tag, cwd, exe, args)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		_ = os.Remove(scriptPath)
		return err
	}
	go func() { _ = cmd.Wait(); _ = os.Remove(scriptPath) }()
	if progress != nil {
		fmt.Fprintln(progress, "Update will install after Solomon exits, then restart in this terminal.")
	}
	return nil
}

func writeWindowsInstallRestartScript(pid int, tag, cwd, exe string, args []string) (string, error) {
	script := fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$ParentPID = %d
$Tag = '%s'
$Cwd = '%s'
$RestartExe = '%s'
$RestartArgs = %s
while (Get-Process -Id $ParentPID -ErrorAction SilentlyContinue) { Start-Sleep -Milliseconds 250 }
if ($Cwd) { Set-Location $Cwd }
$env:SOLOMON_VERSION = $Tag
irm '%s' | iex
& $RestartExe @RestartArgs
`, pid, psQuote(tag), psQuote(cwd), psQuote(exe), psArgList(args), installPS1RawURL)
	return writePowerShellScript(script)
}

func scheduleRestartOnly(ctx context.Context, pid int, cwd, exe string, args []string, progress io.Writer) error {
	switch runtime.GOOS {
	case "linux", "darwin":
		scriptPath, err := writeUnixRestartOnlyScript(pid, cwd, exe, args)
		if err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, "bash", scriptPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			_ = os.Remove(scriptPath)
			return err
		}
		go func() { _ = cmd.Wait(); _ = os.Remove(scriptPath) }()
	case "windows":
		scriptPath, err := writeWindowsRestartOnlyScript(pid, cwd, exe, args)
		if err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			_ = os.Remove(scriptPath)
			return err
		}
		go func() { _ = cmd.Wait(); _ = os.Remove(scriptPath) }()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	if progress != nil {
		fmt.Fprintln(progress, "Restarting Solomon in this terminal...")
	}
	return nil
}

func writeUnixRestartOnlyScript(pid int, cwd, exe string, args []string) (string, error) {
	var b strings.Builder
	b.WriteString("#!/usr/bin/env bash\nset -euo pipefail\n")
	fmt.Fprintf(&b, "PARENT_PID=%d\n", pid)
	fmt.Fprintf(&b, "CWD=%s\n", shellQuote(cwd))
	fmt.Fprintf(&b, "RESTART_EXE=%s\n", shellQuote(exe))
	b.WriteString("while kill -0 \"$PARENT_PID\" 2>/dev/null; do sleep 0.25; done\n")
	b.WriteString("if [[ -n \"$CWD\" ]]; then cd \"$CWD\"; fi\n")
	b.WriteString("RESTART_ARGS=(")
	for i, a := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(shellQuote(a))
	}
	b.WriteString(")\nexec \"$RESTART_EXE\" \"${RESTART_ARGS[@]}\"\n")
	return writeExecutableScript(b.String())
}

func writeWindowsRestartOnlyScript(pid int, cwd, exe string, args []string) (string, error) {
	script := fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$RestartExe = '%s'
$RestartArgs = %s
$Cwd = '%s'
while (Get-Process -Id %d -ErrorAction SilentlyContinue) { Start-Sleep -Milliseconds 250 }
if ($Cwd) { Set-Location $Cwd }
& $RestartExe @RestartArgs
`, psQuote(exe), psArgList(args), psQuote(cwd), pid)
	return writePowerShellScript(script)
}

func writeExecutableScript(body string) (string, error) {
	f, err := os.CreateTemp("", "solomon-install-restart-*.sh")
	if err != nil {
		return "", err
	}
	path := f.Name()
	if _, err := f.WriteString(body); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if err := os.Chmod(path, 0o700); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func writePowerShellScript(body string) (string, error) {
	f, err := os.CreateTemp("", "solomon-install-restart-*.ps1")
	if err != nil {
		return "", err
	}
	path := f.Name()
	if _, err := f.WriteString(body); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func psQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func psArgList(args []string) string {
	var b strings.Builder
	b.WriteString("@(")
	for i, a := range args {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("'")
		b.WriteString(psQuote(a))
		b.WriteString("'")
	}
	b.WriteString(")")
	return b.String()
}
