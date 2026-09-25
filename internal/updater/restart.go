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

var execInstallRestart = defaultExecInstallRestart

func SetExecInstallRestartHook(fn func(context.Context, string) error) func() {
	prev := execInstallRestart
	if fn == nil {
		execInstallRestart = defaultExecInstallRestart
	} else {
		execInstallRestart = fn
	}
	return func() { execInstallRestart = prev }
}

func ExecInstallRestart(ctx context.Context, tag string) error {
	return execInstallRestart(ctx, tag)
}

func defaultExecInstallRestart(ctx context.Context, tag string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return fmt.Errorf("empty release tag")
	}
	switch runtime.GOOS {
	case "linux", "darwin":
		return execUnixInstallRestart(ctx, tag)
	case "windows":
		return nil
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
	writeUnixChecksumVerify(&b)
	b.WriteString("mkdir -p \"$(dirname \"$TARGET\")\"\n")
	b.WriteString("mv \"$tmp\" \"$TARGET\"\n")
	b.WriteString("chmod +x \"$TARGET\"\n")
	b.WriteString("echo \"Installed $TAG -> $TARGET\"\n")
	// The executable captured before the download may be a development or
	// temporary binary. Always restart the freshly installed target instead.
	b.WriteString("RESTART_EXE=\"$TARGET\"\n")
	b.WriteString("echo \"=== Restarting Solomon ===\"\n")
	b.WriteString("echo \"\"\n")
	b.WriteString("stty sane opost onlcr icanon echo 2>/dev/null || true\n")
	writeUnixRestartArgsExec(&b, args, true)
	return writeExecutableScript(b.String())
}

func writeUnixChecksumVerify(b *strings.Builder) {
	b.WriteString("CHECKSUMS_URL=\"https://github.com/SAPPHIR3-ROS3/Solomon/releases/download/${TAG}/checksums.txt\"\n")
	b.WriteString("checksums=\"$(mktemp)\"\n")
	b.WriteString("if curl -fsSL \"$CHECKSUMS_URL\" -o \"$checksums\"; then\n")
	b.WriteString("  expected=\"$(awk -v asset=\"$ASSET\" '$NF==asset {print $1; exit}' \"$checksums\")\"\n")
	b.WriteString("  if [[ -z \"$expected\" ]]; then\n")
	b.WriteString("    echo \"checksums: no entry for $ASSET in checksums.txt\" >&2\n")
	b.WriteString("    rm -f \"$tmp\" \"$checksums\"\n")
	b.WriteString("    exit 1\n")
	b.WriteString("  fi\n")
	b.WriteString("  actual=\"$(sha256sum \"$tmp\" | awk '{print $1}')\"\n")
	b.WriteString("  if [[ \"$expected\" != \"$actual\" ]]; then\n")
	b.WriteString("    echo \"checksum mismatch for $ASSET (expected $expected, got $actual)\" >&2\n")
	b.WriteString("    rm -f \"$tmp\" \"$checksums\"\n")
	b.WriteString("    exit 1\n")
	b.WriteString("  fi\n")
	b.WriteString("  rm -f \"$checksums\"\n")
	b.WriteString("else\n")
	b.WriteString("  echo \"Warning: no checksums.txt for $TAG; skipping integrity check\" >&2\n")
	b.WriteString("fi\n")
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
	cmd := exec.CommandContext(ctx, windowsPowerShellExe(), "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
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
	body, err := windowsInstallRestartScriptBody(pid, tag, cwd, exe, args)
	if err != nil {
		return "", err
	}
	return writePowerShellScript(body)
}

func windowsInstallRestartScriptBody(pid int, tag, cwd, exe string, args []string) (string, error) {
	if strings.TrimSpace(exe) == "" {
		return "", fmt.Errorf("windows install restart: empty executable path")
	}
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
$UpgradeLock = Join-Path ([System.IO.Path]::GetTempPath()) 'solomon-upgrade.lock'
try {
  $null = [System.IO.File]::Open($UpgradeLock, [System.IO.FileMode]::OpenOrCreate, [System.IO.FileAccess]::ReadWrite, [System.IO.FileShare]::None)
} catch {
  exit 0
}
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
%sWrite-Host ''
Write-Host "=== Solomon update ($Tag) ==="
Write-Host "Downloading $Asset..."
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ('solomon-dl-' + [guid]::NewGuid().ToString('N') + '.exe')
Invoke-WebRequest -Uri $DownloadUrl -OutFile $tmp -UseBasicParsing
$ChecksumsUrl = 'https://github.com/SAPPHIR3-ROS3/Solomon/releases/download/' + $Tag + '/checksums.txt'
$checksumsPath = [System.IO.Path]::GetTempFileName()
try {
  Invoke-WebRequest -Uri $ChecksumsUrl -OutFile $checksumsPath -UseBasicParsing
  $expected = $null
  Get-Content $checksumsPath | ForEach-Object {
    if ($_ -match '^(\S+)\s+\*?(.+)$') {
      if ($Matches[2].Trim() -eq $Asset) { $expected = $Matches[1].ToLower() }
    }
  }
  if (-not $expected) { throw "checksums: no entry for $Asset in checksums.txt" }
  $actual = (Get-FileHash $tmp -Algorithm SHA256).Hash.ToLower()
  if ($expected -ne $actual) { throw "checksum mismatch for $Asset (expected $expected, got $actual)" }
} catch [System.Net.WebException] {
  if ($_.Exception.Response -and $_.Exception.Response.StatusCode.value__ -eq 404) {
    Write-Warning "no checksums.txt for $Tag; skipping integrity check"
  } else {
    throw
  }
} finally {
  Remove-Item -Force $checksumsPath -ErrorAction SilentlyContinue
}
New-Item -ItemType Directory -Force -Path (Split-Path $Target) | Out-Null
$staging = "$Target.new"
Copy-Item -Force $tmp $staging
Remove-Item -Force $tmp -ErrorAction SilentlyContinue
%s
$targetFullPath = [System.IO.Path]::GetFullPath($Target)
$blockers = @()
for ($i = 0; $i -lt 120; $i++) {
  $blockers = @(Get-Process -Name 'solomon' -ErrorAction SilentlyContinue | Where-Object {
    if ($_.Id -eq $PID -or -not $_.Path) { $false }
    else { try { [System.IO.Path]::GetFullPath($_.Path).Equals($targetFullPath, [System.StringComparison]::OrdinalIgnoreCase) } catch { $false } }
  })
  if ($blockers.Count -eq 0) { break }
  Start-Sleep -Milliseconds 250
}
if ($blockers.Count -gt 0) {
  $blockingPids = ($blockers | ForEach-Object { $_.Id }) -join ', '
  if ($ServerRestart) {
    Write-Warning 'Another Solomon process is still active; restoring the stopped server before aborting.'
    Start-SavedSolomonServer
  }
  throw "cannot update while Solomon is still running from $Target (PID $blockingPids)"
}
$installed = $false
$lastReplaceError = $null
$backup = "$staging.$PID.bak"
for ($i = 0; $i -lt 120; $i++) {
  try {
    if (Test-Path $Target) {
      Remove-Item -Force $backup -ErrorAction SilentlyContinue
      [System.IO.File]::Replace($staging, $Target, $backup)
      Remove-Item -Force $backup -ErrorAction SilentlyContinue
    } else {
      Move-Item -Force $staging $Target -ErrorAction Stop
    }
    $installed = $true
    break
  } catch {
    $lastReplaceError = $_.Exception.Message
    Start-Sleep -Milliseconds 500
  }
}
if (-not $installed) {
  $replaceError = "failed to replace $Target after the download was verified; Windows may still be scanning or locking the file"
  $blockers = @(Get-Process -Name 'solomon' -ErrorAction SilentlyContinue | Where-Object {
    if ($_.Id -eq $PID -or -not $_.Path) { $false }
    else { try { [System.IO.Path]::GetFullPath($_.Path).Equals($targetFullPath, [System.StringComparison]::OrdinalIgnoreCase) } catch { $false } }
  })
  if ($blockers.Count -gt 0) {
    $blockingPids = ($blockers | ForEach-Object { $_.Id }) -join ', '
    $replaceError = "cannot replace $Target; Solomon is still running (PID $blockingPids)"
  } elseif ($lastReplaceError) {
    $replaceError = "$replaceError. Last Windows error: $lastReplaceError"
  }
  if ($ServerRestart) {
    Write-Warning 'The executable was not replaced; restoring the stopped Solomon server.'
    Start-SavedSolomonServer
  }
  throw $replaceError
}
$RestartExe = $Target
Write-Host "Installed $Tag -> $Target"
Start-SavedSolomonServer
Write-Host '=== Restarting Solomon ==='
Write-Host ''
%s
	`, pid, psQuote(tag), psQuote(cwd), psQuote(exe), psArgList(args), psQuote(asset), psQuote(url), psQuote(target), windowsProfileSetupScriptLines(), windowsStartManagedServerFunction()+"\n"+windowsStopManagedServerScript(), restartLine)
	return script, nil
}

func windowsStopManagedServerScript() string {
	return `# Stop the detached server before replacing its executable, then keep its startup settings for after the update.
$ServerRestart = $null
$SolomonHome = if ($env:SOLOMON_HOME) { $env:SOLOMON_HOME } else { Join-Path $env:USERPROFILE '.solomon' }
$ServerStatePath = Join-Path $SolomonHome 'run/server/state.json'
if (Test-Path $ServerStatePath) {
  try { $ServerState = Get-Content -Raw $ServerStatePath | ConvertFrom-Json } catch { $ServerState = $null }
  if ($ServerState -and [int]$ServerState.pid -gt 0) {
    $ServerProcess = Get-Process -Id ([int]$ServerState.pid) -ErrorAction SilentlyContinue
    if ($ServerProcess -and $ServerProcess.Path) {
      try {
        $ServerProcessPath = [System.IO.Path]::GetFullPath($ServerProcess.Path)
        $TargetFullPath = [System.IO.Path]::GetFullPath($Target)
      } catch {
        $ServerProcessPath = ''
      }
      if ($ServerProcessPath -and $ServerProcessPath.Equals($TargetFullPath, [System.StringComparison]::OrdinalIgnoreCase)) {
        $OtherSolomonProcesses = @(Get-Process -Name 'solomon' -ErrorAction SilentlyContinue | Where-Object {
          if ($_.Id -eq $PID -or $_.Id -eq [int]$ServerState.pid -or -not $_.Path) { $false }
          else { try { [System.IO.Path]::GetFullPath($_.Path).Equals($TargetFullPath, [System.StringComparison]::OrdinalIgnoreCase) } catch { $false } }
        })
        if ($OtherSolomonProcesses.Count -gt 0) {
          $blockingPids = ($OtherSolomonProcesses | ForEach-Object { $_.Id }) -join ', '
          throw "cannot update while other Solomon instances are running from $Target (PID $blockingPids); the server was left running"
        }
        $ServerUri = [Uri]$ServerState.url
        if ($ServerUri.Scheme -ne 'http' -or -not $ServerUri.IsLoopback) {
          throw "refusing to stop Solomon server at unexpected URL $($ServerState.url)"
        }
        Write-Host "Stopping Solomon server (PID $($ServerState.pid)) before updating..."
        try {
          $StopResponse = Invoke-WebRequest -Uri ($ServerUri.AbsoluteUri.TrimEnd('/') + '/_solomon/stop') -Method Post -TimeoutSec 5 -UseBasicParsing
        } catch {
          throw "could not stop Solomon server PID $($ServerState.pid): $($_.Exception.Message)"
        }
        if ([int]$StopResponse.StatusCode -ne 202) {
          throw "Solomon server PID $($ServerState.pid) refused to stop (HTTP $($StopResponse.StatusCode))"
        }
        $ServerRestart = $ServerState
        $StopDeadline = [DateTime]::UtcNow.AddSeconds(15)
        while ($true) {
          $ServerProcess = Get-Process -Id ([int]$ServerState.pid) -ErrorAction SilentlyContinue
          if (-not $ServerProcess -or -not $ServerProcess.Path) { break }
          try { $ServerProcessPath = [System.IO.Path]::GetFullPath($ServerProcess.Path) } catch { break }
          if (-not $ServerProcessPath.Equals($TargetFullPath, [System.StringComparison]::OrdinalIgnoreCase)) { break }
          if ([DateTime]::UtcNow -ge $StopDeadline) {
            throw "Solomon server PID $($ServerState.pid) did not exit after its graceful stop request"
          }
          Start-Sleep -Milliseconds 250
        }
      }
    }
  }
}`
}

func windowsStartManagedServerFunction() string {
	return `function Start-SavedSolomonServer {
  if (-not $ServerRestart) { return }
  try {
    $ServerStartArgs = @('server', 'start')
    if ($ServerRestart.mode -eq 'dev') {
      if (-not $ServerRestart.dev_directory) { throw 'saved dev server state has no GUI directory' }
      $ServerStartArgs += @('dev', [string]$ServerRestart.dev_directory)
    }
    Write-Host 'Restarting Solomon server...'
    & $Target @ServerStartArgs
    $ServerStarted = $false
    for ($i = 0; $i -lt 100; $i++) {
      try {
        if (Test-Path $ServerStatePath) {
          $StartedState = Get-Content -Raw $ServerStatePath | ConvertFrom-Json
          $RestartedAfterStop = [DateTime]::Parse([string]$StartedState.started_at).ToUniversalTime() -gt [DateTime]::Parse([string]$ServerRestart.started_at).ToUniversalTime()
          if ([int]$StartedState.pid -gt 0 -and $RestartedAfterStop) {
            $StartedUri = [Uri]$StartedState.url
            if ($StartedUri.IsLoopback) {
              $Health = Invoke-RestMethod -Uri ($StartedUri.AbsoluteUri.TrimEnd('/') + '/health') -TimeoutSec 1
              if ($Health.ok) { $ServerStarted = $true; break }
            }
          }
        }
      } catch {}
      Start-Sleep -Milliseconds 100
    }
    if ($ServerStarted) {
      Write-Host 'Solomon server restarted.'
    } else {
      Write-Warning 'The previous Solomon server did not restart automatically. Run solomon server start.'
    }
  } catch {
    Write-Warning "The previous Solomon server could not be restarted automatically: $_"
  }
}`
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
		cmd := exec.CommandContext(ctx, windowsPowerShellExe(), "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
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
	if strings.TrimSpace(exe) == "" {
		return "", fmt.Errorf("windows restart: empty executable path")
	}
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
