package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUnixInstallerReportsEveryFailedStage(t *testing.T) {
	stages := []struct{ function, label string }{
		{"ensure_homebrew", "Homebrew setup"},
		{"ensure_go", "Go toolchain setup"},
		{"ensure_make", "GNU Make setup"},
		{"install_cloakbrowser", "CloakBrowser setup"},
		{"setup_shell", "Shell PATH and profile setup"},
		{"configure_runtime_defaults", "Runtime configuration"},
		{"install_solomon", "Solomon CLI and desktop setup"},
	}
	for _, stage := range stages {
		for _, mode := range []string{"return", "exit", "subshell", "substitution"} {
			t.Run(stage.function+"/"+mode, func(t *testing.T) {
				command := exec.Command(installerBash(t), "-c", `
source "$1"
failed="$2"; mode="$3"
uname() { echo Darwin; }
simulate() {
  echo "RUN $1"
  if [[ "$1" == "$failed" ]]; then
    INSTALL_OPERATION="Fixture operation: $1"
    echo 'Underlying error: fixture permission denied' >&2
    case "$mode" in
      return) return 42 ;;
      exit) exit 42 ;;
      subshell) (exit 42) ;;
      substitution) value="$(exit 42)" ;;
    esac
  fi
}

ensure_homebrew() { simulate ensure_homebrew; }
ensure_go() { simulate ensure_go; }
ensure_make() { simulate ensure_make; }
install_cloakbrowser() { simulate install_cloakbrowser; }
setup_shell() { simulate setup_shell; }
configure_runtime_defaults() { simulate configure_runtime_defaults; }
install_solomon() { simulate install_solomon; }
main
`, "installer-diagnostics", filepath.ToSlash(installerScriptFixture(t)), stage.function, mode)
				output, err := command.CombinedOutput()
				if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 42 {
					t.Fatalf("expected original exit code 42, got %v\n%s", err, output)
				}
				for _, expected := range []string{"[install] FAILED: " + stage.label, "Operation:", "Exit code: 42", "Next:", "Underlying error: fixture permission denied"} {
					if !strings.Contains(string(output), expected) {
						t.Fatalf("missing %q\n%s", expected, output)
					}
				}
				if strings.Contains(string(output), "Done.") {
					t.Fatalf("reported success after failure\n%s", output)
				}
			})
		}
	}
}

func TestWindowsInstallerReportsOriginalFailureAndStops(t *testing.T) {
	runWindowsInstallerFixture(t, `param($Installer, $Fixture)
$ErrorActionPreference = 'Stop'
$tokens = $null; $errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($Installer, [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
foreach ($name in @('Invoke-InstallerStep', 'Ensure-Node', 'Get-GoInstallBinDir')) {
 $definition = $ast.Find({param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name}, $true)
 Invoke-Expression $definition.Extent.Text
}
$failure = $null
try {
 Invoke-InstallerStep 'Fixture stage' 'Fixture fix' {
  $script:InstallOperation = 'Write fixture path'
  throw 'Original access denied detail'
 }
 throw 'Failure was ignored'
} catch { $failure = $_.Exception.Message }
foreach ($expected in @('FAILED: Fixture stage', 'Operation: Write fixture path', 'Cause: Original access denied detail', 'Next: Fixture fix')) {
 if (-not $failure.Contains($expected)) { throw "Missing diagnostic: $expected in $failure" }
}
function Get-Command { param($Name, $ErrorAction); if ($Name -eq 'winget') { [pscustomobject]@{Source='winget.exe'} } }
function winget { Write-Host 'Original winget source unavailable'; $global:LASTEXITCODE = -1978335212 }
try { Ensure-Node; throw 'winget failure ignored' }
catch { if ($_.Exception.Message -notlike '*winget failed (exit code -1978335212)*') { throw } }
function go { Write-Host 'Original Go environment failure'; $global:LASTEXITCODE = 17 }
try { Get-GoInstallBinDir; throw 'Go failure ignored' }
catch { if ($_.Exception.Message -notlike '*go env GOBIN failed (exit code 17)*') { throw } }
`)
}

func TestLinuxDesktopInstallerReportsMissingBuildAndPreservesInstallation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux desktop installer")
	}
	fixture := t.TempDir()
	script := filepath.Join(fixture, "scripts", "install-desktop.sh")
	bin := filepath.Join(fixture, "bin")
	for _, dir := range []string{filepath.Dir(script), bin} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	source, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "install-desktop.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string][]byte{
		script:                                source,
		filepath.Join(bin, "solomon"):         []byte("fixture CLI"),
		filepath.Join(bin, "solomon-desktop"): []byte("previous desktop"),
	} {
		if err := os.WriteFile(path, content, 0700); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command(installerBash(t), script, bin)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("missing build was accepted\n%s", output)
	}
	for _, expected := range []string{"FAILED: Linux desktop installation", "Operation: Install desktop binary", "Exit code:", "solomon-desktop", "Next:"} {
		if !strings.Contains(string(output), expected) {
			t.Fatalf("missing diagnostic %q\n%s", expected, output)
		}
	}
	previous, err := os.ReadFile(filepath.Join(bin, "solomon-desktop"))
	if err != nil || string(previous) != "previous desktop" {
		t.Fatalf("previous desktop changed: %q, %v", previous, err)
	}
	leftovers, err := filepath.Glob(filepath.Join(bin, ".solomon-desktop.*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary file cleanup failed: %v, %v", leftovers, err)
	}
}

func TestUnixInstallerDownloadFailurePreservesCauseAndExitCode(t *testing.T) {
	command := exec.Command(installerBash(t), "-c", `
source "$1"
INSTALL_VERSION=v2099.1.0
fixture="$2"
mkdir -p "$fixture/bin"
printf 'previous CLI' > "$fixture/bin/solomon"
detect_platform() { echo linux-amd64; }
go_install_bin_dir() { echo "$fixture/bin"; }
curl() { echo 'curl: (28) Connection timed out' >&2; return 28; }
sleep() { :; }
installer_enable_diagnostics
installer_step 'Solomon CLI and desktop setup' 'Check the reported URL and connection.'
install_release_asset
`, "installer-download-diagnostics", filepath.ToSlash(installerScriptFixture(t)), filepath.ToSlash(t.TempDir()))
	output, err := command.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 28 {
		t.Fatalf("expected curl exit code 28: %v\n%s", err, output)
	}
	for _, expected := range []string{"Connection timed out", "15 attempts (curl exit code 28)", "https://github.com/SAPPHIR3-ROS3/Solomon/releases/download/v2099.1.0/solomon-v2099.1.0-linux-amd64", "FAILED: Solomon CLI and desktop setup"} {
		if !strings.Contains(string(output), expected) {
			t.Fatalf("missing diagnostic %q\n%s", expected, output)
		}
	}
}

func TestWindowsInstallerDownloadFailurePreservesOriginalException(t *testing.T) {
	runWindowsInstallerFixture(t, `param($Installer, $Fixture)
$ErrorActionPreference = 'Stop'
$tokens = $null; $errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($Installer, [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
$definition = $ast.Find({param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Install-ReleaseAsset'}, $true)
Invoke-Expression $definition.Extent.Text
$Version = 'v2099.1.0'
$script:attempts = 0
function Get-GoArch { 'amd64' }
function Get-GoInstallBinDir { $Fixture }
function Start-Sleep {}
function Invoke-WebRequest { $script:attempts++; throw 'HTTP 403: API rate limit exceeded' }
$failure = $null
try { Install-ReleaseAsset; throw 'Download failure ignored' }
catch { $failure = $_.Exception.Message }
foreach ($expected in @('15 attempts', 'HTTP 403: API rate limit exceeded', 'https://github.com/SAPPHIR3-ROS3/Solomon/releases/download/v2099.1.0/solomon-v2099.1.0-windows-amd64.exe')) {
 if (-not $failure.Contains($expected)) { throw "Missing diagnostic: $expected in $failure" }
}
if ($script:attempts -ne 15) { throw 'Incorrect retry count' }
`)
}
