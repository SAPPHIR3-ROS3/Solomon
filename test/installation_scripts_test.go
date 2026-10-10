package test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func installerBash(t *testing.T) string {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if runtime.GOOS == "windows" {
		bash = filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe")
		_, err = os.Stat(bash)
	}
	if err != nil {
		t.Skip("Bash unavailable")
	}
	return bash
}

func installerScriptFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "installer.sh")
	if err := os.WriteFile(file, []byte(strings.ReplaceAll(string(data), "\r\n", "\n")), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestMacOSInstallerUsesHomebrewAndPersistsNodePath(t *testing.T) {
	for _, scenario := range []string{"lts", "fallback", "failed install", "broken node", "missing npm"} {
		t.Run(scenario, func(t *testing.T) {
			command := exec.Command(installerBash(t), "-c", `
source "$1"
# PATH entries need MSYS paths on Windows; drive-letter colons split PATH.
fixture_dir="$(cd "$2" && pwd)"; fixture_scenario="$3"
mkdir -p "$fixture_dir/old/bin" "$fixture_dir/brew/bin" "$fixture_dir/node@24/bin" "$fixture_dir/node/bin"
printf '#!/usr/bin/env bash\necho v16.0.0\n' > "$fixture_dir/old/bin/node"
if [[ "$fixture_scenario" == 'broken node' ]]; then
  printf '#!/usr/bin/env bash\nexit 1\n' > "$fixture_dir/old/bin/node"
fi
for formula in node@24 node; do
  printf '#!/usr/bin/env bash\necho v24.0.0\n' > "$fixture_dir/$formula/bin/node"
  printf '#!/usr/bin/env bash\necho 10.0.0\n' > "$fixture_dir/$formula/bin/npm"
  chmod +x "$fixture_dir/$formula/bin/"*
done
if [[ "$fixture_scenario" == 'missing npm' ]]; then
  printf '#!/usr/bin/env bash\necho v24.0.0\n' > "$fixture_dir/old/bin/node"
fi
chmod +x "$fixture_dir/old/bin/node"
export PATH="$fixture_dir/old/bin:$PATH"
node_lts_major() { echo 24; }
uname() { echo Darwin; }
port() { echo 'MacPorts must not be used' >&2; exit 90; }
sudo() { echo 'sudo must not be used for formula installation' >&2; exit 91; }
brew() {
  printf '%s\n' "$*" >> "$fixture_dir/brew.log"
  case "$1" in
    --prefix) printf '%s\n' "$fixture_dir/${2:-brew}" ;;
    install)
      [[ "$fixture_scenario" != 'failed install' ]] || return 1
      if [[ "$fixture_scenario" == fallback && "$2" == node@24 ]]; then return 1; fi ;;
    *) echo 'Unexpected brew command' >&2; exit 92 ;;
  esac
}
# Simulate no npm in the original runtime, even on hosts with npm installed.
command() {
  if [[ "$fixture_scenario" == 'missing npm' && "${1:-}" == -v && "${2:-}" == npm && "$PATH" == "$fixture_dir/old/bin:"* ]]; then
    return 1
  fi
  builtin command "$@"
}
if [[ "$fixture_scenario" == 'failed install' ]]; then
  if install_node_darwin; then echo 'Failed brew install accepted' >&2; exit 93; fi
  exit 0
fi
ensure_node
[[ "$(node --version)" == v24.0.0 ]]
command -v npm
expected=node@24
[[ "$fixture_scenario" != fallback ]] || expected=node
[[ "$NODE_BIN_DIR" == "$fixture_dir/$expected/bin" ]]
rc="$fixture_dir/profile"
configure_unix_rc "$rc"
configure_unix_rc "$rc"
[[ "$(grep -c '^# solomon-installer-dependencies$' "$rc")" == 2 ]]
export PATH="$fixture_dir/old/bin:$PATH"
source "$rc"
[[ "$(command -v node)" == "$fixture_dir/$expected/bin/node" ]]
configure_fish_rc "$fixture_dir/config.fish"
grep -q 'fish_add_path --prepend .*node' "$fixture_dir/config.fish"
grep -q 'install node@24' "$fixture_dir/brew.log"
`, "installer-test", filepath.ToSlash(installerScriptFixture(t)), filepath.ToSlash(t.TempDir()), scenario)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("macOS Node installation: %v\n%s", err, output)
			}
		})
	}
}

func TestUnixInstallerBootstrapsHomebrewAtStartOnMacOS(t *testing.T) {
	for _, scenario := range []string{"Darwin", "Linux", "bootstrap failure"} {
		t.Run(scenario, func(t *testing.T) {
			command := exec.Command(installerBash(t), "-c", `
source "$1"
scenario="$2"; steps=""
uname() { if [[ "$scenario" == Linux ]]; then echo Linux; else echo Darwin; fi; }
ensure_homebrew() {
  steps="${steps}brew "
  [[ "$scenario" != 'bootstrap failure' ]]
}
ensure_go() { steps="${steps}go "; }
ensure_make() { steps="${steps}make "; }
install_cloakbrowser() { steps="${steps}browser "; }
setup_shell() { steps="${steps}path "; }
configure_runtime_defaults() { steps="${steps}config "; }
install_solomon() { steps="${steps}solomon "; }
if [[ "$scenario" == 'bootstrap failure' ]]; then
  if main; then exit 7; fi
  [[ "$steps" == 'brew ' ]]
else
  main
  expected='go make browser path config solomon '
  [[ "$scenario" != Darwin ]] || expected="brew $expected"
  [[ "$steps" == "$expected" ]]
fi
`, "installer-test", filepath.ToSlash(installerScriptFixture(t)), scenario)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("installer bootstrap order: %v\n%s", err, output)
			}
		})
	}
}

func TestMacOSHomebrewBootstrapFailureStopsBeforeInstallingNode(t *testing.T) {
	command := exec.Command(installerBash(t), "-c", `
source "$1"
find_homebrew() { return 1; }
curl() {
  [[ "$2" == https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh ]]
  printf 'exit 1\n' > "$4"
}
brew() { echo 'brew must not run after bootstrap failure' >&2; exit 90; }
install_node_darwin
`, "installer-test", filepath.ToSlash(installerScriptFixture(t)))
	// A failed bootstrap must propagate failure and provide recovery instructions.
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "Homebrew installation failed") {
		t.Fatalf("bootstrap failure: %v\n%s", err, output)
	}
}

func TestUnixInstallerWorksWithoutGNUVersionSortAndPreservesTomlTables(t *testing.T) {
	script := installerScriptFixture(t)
	config := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(config, []byte("server_port = 1\n[other]\nserver_port = 9\n"), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(installerBash(t), "-c", `
source "$1"
sort() { echo 'BSD sort does not support -V' >&2; return 2; }
version_ge 1.25.0 1.25.0
version_ge 1.27.0 1.25.0
version_ge 24.10.0 24.9.0
if version_ge 20.9.0 20.10.0; then exit 7; fi
upsert_toml_scalar "$2" server_port 64000
`, "installer-test", filepath.ToSlash(script), filepath.ToSlash(config))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Unix helpers: %v\n%s", err, output)
	}
	data, _ := os.ReadFile(config)
	if string(data) != "server_port = 64000\n[other]\nserver_port = 9\n" {
		t.Fatalf("installer changed a table's setting: %s", data)
	}
}

func TestUnixReleaseInstallerVerifiesBeforeReplacingAndCleansDownloads(t *testing.T) {
	const payload = "verified release fixture\n"
	for _, scenario := range []string{"success", "bad checksum", "missing checksums"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "binary directory with spaces")
			if err := os.Mkdir(bin, 0755); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(bin, "solomon")
			if err := os.WriteFile(target, []byte("previous binary"), 0755); err != nil {
				t.Fatal(err)
			}
			fixture := filepath.Join(dir, "payload")
			if err := os.WriteFile(fixture, []byte(payload), 0600); err != nil {
				t.Fatal(err)
			}
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte(payload)))
			if scenario == "bad checksum" {
				hash = strings.Repeat("0", 64)
			}
			command := exec.Command(installerBash(t), "-c", `
source "$1"
INSTALL_VERSION=v2099.1.0
# Preserve test paths in globals; functions have their own positional arguments.
fixture_bin="$2"; fixture_payload="$3"; fixture_hash="$4"; fixture_scenario="$5"
go_install_bin_dir() { printf '%s\n' "$fixture_bin"; }
detect_platform() { echo darwin-arm64; }
curl() {
  local url="$2" output="$4"
  if [[ "$url" == */checksums.txt ]]; then
    [[ "$fixture_scenario" != 'missing checksums' ]] || return 22
    printf '%s *solomon-v2099.1.0-darwin-arm64\n' "$fixture_hash" > "$output"
  else
    cp "$fixture_payload" "$output"
  fi
}
install_release_asset
`, "installer-test", filepath.ToSlash(installerScriptFixture(t)), filepath.ToSlash(bin), filepath.ToSlash(fixture), hash, scenario)
			output, err := command.CombinedOutput()
			if (err != nil) != (scenario != "success") {
				t.Fatalf("scenario=%s err=%v\n%s", scenario, err, output)
			}
			want := "previous binary"
			if scenario == "success" {
				want = payload
			}
			got, err := os.ReadFile(target)
			if err != nil || string(got) != want {
				t.Fatalf("target=%q err=%v, want %q", got, err, want)
			}
			entries, err := os.ReadDir(bin)
			if err != nil || len(entries) != 1 {
				t.Fatalf("download leftovers: %v %v", entries, err)
			}
		})
	}
}

func TestCachedUnixBrowserRequiresWorkingSupportedNode(t *testing.T) {
	dir := t.TempDir()
	cloak := filepath.Join(dir, "cloakbrowser")
	for _, name := range []string{"cloakbrowser", "playwright-core"} {
		pkg := filepath.Join(cloak, "node_modules", name, "package.json")
		if err := os.MkdirAll(filepath.Dir(pkg), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(pkg, []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	browser := filepath.Join(cloak, "cache", "chrome")
	if err := os.MkdirAll(filepath.Dir(browser), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(browser, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(installerBash(t), "-c", `
source "$1"
export SOLOMON_HOME="$2"
uname() { echo Linux; }
node() { echo "$fixture_node_version"; return "$fixture_node_exit"; }
fixture_node_version=v16.0.0; fixture_node_exit=0
if cloakbrowser_ready; then echo 'Cached browser accepted unsupported Node' >&2; exit 7; fi
fixture_node_version=v24.0.0; fixture_node_exit=1
if cloakbrowser_ready; then echo 'Cached browser accepted broken Node' >&2; exit 8; fi
fixture_node_exit=0
cloakbrowser_ready
`, "installer-test", filepath.ToSlash(installerScriptFixture(t)), filepath.ToSlash(dir))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("cached browser Node check: %v\n%s", err, output)
	}
}

func TestPowerShellInstallerPreservesTomlTablesAndCleansFailedDownloads(t *testing.T) {
	powershell, err := exec.LookPath("powershell")
	if err != nil {
		powershell, err = exec.LookPath("pwsh")
	}
	if err != nil {
		t.Skip("PowerShell unavailable")
	}
	dir := t.TempDir()
	fixture := filepath.Join(dir, "check.ps1")
	body := `param([string]$Installer, [string]$Fixture)
$ErrorActionPreference = 'Stop'
$tokens = $null; $errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($Installer, [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
foreach ($name in @('Set-TomlScalar', 'Get-SolomonTempDir', 'Install-ReleaseAsset', 'Install-Solomon', 'Test-VersionGe', 'Test-CloakBrowserReady')) {
  $function = $ast.Find({param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name}, $true)
  Invoke-Expression $function.Extent.Text
}
$config = Join-Path $Fixture 'config.toml'
[IO.File]::WriteAllText($config, "server_port = 1` + "`n" + `[other]` + "`n" + `server_port = 9` + "`n" + `")
Set-TomlScalar $config server_port '64000'
$data = [IO.File]::ReadAllText($config) -replace "` + "`r`n" + `", "` + "`n" + `"
if ($data -ne "server_port = 64000` + "`n" + `[other]` + "`n" + `server_port = 9` + "`n" + `") { throw 'Table setting was removed' }
$script:Version = 'v2099.1.0'
function Get-GoArch { 'amd64' }
function Get-GoInstallBinDir { $Fixture }
function Invoke-WebRequest {
  param($Uri, $OutFile, [switch]$UseBasicParsing)
  if ($Uri -like '*/checksums.txt') { throw 'offline checksum download' }
  [IO.File]::WriteAllText($OutFile, 'unverified new binary')
}
$original = Join-Path $Fixture 'solomon.exe'
[IO.File]::WriteAllText($original, 'previous binary')
try { Install-ReleaseAsset; throw 'Unverified installation was accepted' }
catch { if ($_.Exception.Message -notlike '*offline checksum*') { throw } }
if ([IO.File]::ReadAllText($original) -ne 'previous binary') { throw 'Original binary changed' }
if (Get-ChildItem -LiteralPath $Fixture -Filter '.solomon-download-*') { throw 'Failed download leaked' }
$env:SOLOMON_HOME = Join-Path $Fixture 'browser home'
$NodeRequired = '20.0.0'
$cloak = Join-Path $env:SOLOMON_HOME 'cloakbrowser'
foreach ($package in @('cloakbrowser', 'playwright-core')) {
  $path = Join-Path $cloak "node_modules/$package/package.json"
  New-Item -ItemType Directory -Force -Path (Split-Path $path -Parent) | Out-Null
  [IO.File]::WriteAllText($path, '{}')
}
New-Item -ItemType Directory -Force -Path (Join-Path $cloak 'cache') | Out-Null
[IO.File]::WriteAllText((Join-Path $cloak 'cache/chrome.exe'), 'cached browser')
function node { $global:LASTEXITCODE = $script:fixtureNodeExit; $script:fixtureNodeVersion }
$script:fixtureNodeVersion = 'v16.0.0'; $script:fixtureNodeExit = 0
if (Test-CloakBrowserReady) { throw 'Cached browser accepted unsupported Node' }
$script:fixtureNodeVersion = 'v24.0.0'; $script:fixtureNodeExit = 1
if (Test-CloakBrowserReady) { throw 'Cached browser accepted broken Node' }
$script:fixtureNodeExit = 0
if (-not (Test-CloakBrowserReady)) { throw 'Valid cached browser was rejected' }
function Resolve-InstallVersion {}
function Ensure-GoBinInPath {}
function Get-CimInstance {}
function Install-ReleaseAsset { [IO.File]::WriteAllText($original, 'invalid executable causing setup failure') }
try { Install-Solomon; throw 'Failed desktop setup was accepted' }
catch { if ($_.Exception.Message -eq 'Failed desktop setup was accepted') { throw } }
if ([IO.File]::ReadAllText($original) -ne 'previous binary') { throw 'Failed desktop setup lost the previous CLI' }
`
	if err := os.WriteFile(fixture, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(powershell, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", fixture, filepath.Join(repoRoot(t), "scripts", "install.ps1"), dir)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("PowerShell installer regression: %v\n%s", err, output)
	}
}
