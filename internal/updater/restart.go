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
	"syscall"
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

func ExecInstallRestart(ctx context.Context, tag string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return fmt.Errorf("empty release tag")
	}
	switch runtime.GOOS {
	case "linux", "darwin":
		return execUnixInstallRestart(ctx, tag)
	case "windows":
		return launchWindowsInstallRestart(ctx, os.Getpid(), tag, "", "", nil, io.Discard)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func execUnixInstallRestart(ctx context.Context, tag string) error {
	_ = ctx
	exe, err := restartExecutable()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	args := append([]string(nil), os.Args[1:]...)
	scriptPath, err := writeUnixInstallRestartScript(0, tag, cwd, exe, args)
	if err != nil {
		return err
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		_ = os.Remove(scriptPath)
		return err
	}
	if err := syscall.Exec(bash, []string{"bash", scriptPath}, os.Environ()); err != nil {
		_ = os.Remove(scriptPath)
		return err
	}
	return nil
}

func launchUnixInstallRestart(ctx context.Context, pid int, tag, cwd, exe string, args []string, progress io.Writer) error {
	_ = ctx
	_ = progress
	scriptPath, err := writeUnixInstallRestartScript(pid, tag, cwd, exe, args)
	if err != nil {
		return err
	}
	cmd := exec.Command("bash", scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		_ = os.Remove(scriptPath)
		return err
	}
	go func() { _ = cmd.Wait(); _ = os.Remove(scriptPath) }()
	return nil
}

func writeUnixInstallRestartScript(pid int, tag, cwd, exe string, args []string) (string, error) {
	asset, err := releaseAssetName(tag)
	if err != nil {
		return "", err
	}
	target, err := installTargetPath()
	if err != nil {
		return "", err
	}
	url := releaseDownloadURL(tag, asset)
	var b strings.Builder
	b.WriteString("#!/usr/bin/env bash\nset -euo pipefail\n")
	fmt.Fprintf(&b, "PARENT_PID=%d\n", pid)
	fmt.Fprintf(&b, "TAG=%s\n", shellQuote(tag))
	fmt.Fprintf(&b, "CWD=%s\n", shellQuote(cwd))
	fmt.Fprintf(&b, "RESTART_EXE=%s\n", shellQuote(exe))
	fmt.Fprintf(&b, "ASSET=%s\n", shellQuote(asset))
	fmt.Fprintf(&b, "DOWNLOAD_URL=%s\n", shellQuote(url))
	fmt.Fprintf(&b, "TARGET=%s\n", shellQuote(target))
	if pid > 0 {
		b.WriteString("while kill -0 \"$PARENT_PID\" 2>/dev/null; do sleep 0.25; done\n")
	}
	b.WriteString("if [[ -n \"$CWD\" ]]; then cd \"$CWD\"; fi\n")
	b.WriteString("echo \"\"\n")
	b.WriteString("echo \"=== Solomon update ($TAG) ===\"\n")
	b.WriteString("echo \"Downloading $ASSET...\"\n")
	b.WriteString("tmp=\"$(mktemp)\"\n")
	b.WriteString("curl -fsSL \"$DOWNLOAD_URL\" -o \"$tmp\"\n")
	b.WriteString("mkdir -p \"$(dirname \"$TARGET\")\"\n")
	b.WriteString("mv \"$tmp\" \"$TARGET\"\n")
	b.WriteString("chmod +x \"$TARGET\"\n")
	b.WriteString("echo \"Installed $TAG -> $TARGET\"\n")
	b.WriteString("echo \"=== Restarting Solomon ===\"\n")
	b.WriteString("echo \"\"\n")
	b.WriteString("stty sane opost onlcr icanon echo 2>/dev/null || true\n")
	writeUnixRestartArgsExec(&b, args, true)
	return writeExecutableScript(b.String())
}

func writeUnixRestartArgsExec(b *strings.Builder, args []string, ttyStdin bool) {
	b.WriteString("RESTART_ARGS=(")
	for i, a := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(shellQuote(a))
	}
	b.WriteString(")\n")
	redirect := ""
	if ttyStdin {
		redirect = " </dev/tty"
	}
	b.WriteString("if ((${#RESTART_ARGS[@]} > 0)); then\n")
	b.WriteString("  exec \"$RESTART_EXE\" \"${RESTART_ARGS[@]}\"" + redirect + "\n")
	b.WriteString("else\n")
	b.WriteString("  exec \"$RESTART_EXE\"" + redirect + "\n")
	b.WriteString("fi\n")
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
	return nil
}

func writeWindowsInstallRestartScript(pid int, tag, cwd, exe string, args []string) (string, error) {
	asset, err := releaseAssetName(tag)
	if err != nil {
		return "", err
	}
	target, err := installTargetPath()
	if err != nil {
		return "", err
	}
	url := releaseDownloadURL(tag, asset)
	restartLine := "& $RestartExe"
	if len(args) > 0 {
		restartLine = "& $RestartExe @RestartArgs"
	}
	script := fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$ParentPID = %d
$Tag = '%s'
$Cwd = '%s'
$RestartExe = '%s'
$RestartArgs = %s
$Asset = '%s'
$DownloadUrl = '%s'
$Target = '%s'
while (Get-Process -Id $ParentPID -ErrorAction SilentlyContinue) { Start-Sleep -Milliseconds 250 }
if ($Cwd) { Set-Location $Cwd }
Write-Host ''
Write-Host "=== Solomon update ($Tag) ==="
Write-Host "Downloading $Asset..."
$tmp = [System.IO.Path]::GetTempFileName()
Invoke-WebRequest -Uri $DownloadUrl -OutFile $tmp -UseBasicParsing
New-Item -ItemType Directory -Force -Path (Split-Path $Target) | Out-Null
Move-Item -Force $tmp $Target
Write-Host "Installed $Tag -> $Target"
Write-Host '=== Restarting Solomon ==='
Write-Host ''
%s
`, pid, psQuote(tag), psQuote(cwd), psQuote(exe), psArgList(args), psQuote(asset), psQuote(url), psQuote(target), restartLine)
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
	writeUnixRestartArgsExec(&b, args, false)
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
