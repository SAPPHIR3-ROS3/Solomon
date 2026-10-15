package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func installerPowerShell(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"powershell", "pwsh"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	t.Skip("PowerShell unavailable")
	return ""
}

func runWindowsInstallerFixture(t *testing.T, body string, args ...string) {
	t.Helper()
	fixture := filepath.Join(t.TempDir(), "check.ps1")
	if err := os.WriteFile(fixture, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	commandArgs := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", fixture, filepath.Join(repoRoot(t), "scripts", "install.ps1"), t.TempDir()}
	command := exec.Command(installerPowerShell(t), append(commandArgs, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Windows installer regression: %v\n%s", err, output)
	}
}

const windowsInstallerFixtureFunctions = `
$ErrorActionPreference = 'Stop'
$tokens = $null; $errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($Installer, [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
foreach ($name in @('Ensure-Make', 'Update-InstallerProcessPath')) {
 $definition = $ast.Find({param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name}, $true)
 if (-not $definition) { throw "Missing function $name" }
 # Stub only the registry boundary; never modify the host user/machine PATH.
 $text = $definition.Extent.Text.Replace("[Environment]::GetEnvironmentVariable('Path', 'User')", '$script:fixtureUserPath').Replace("[Environment]::GetEnvironmentVariable('Path', 'Machine')", '$script:fixtureMachinePath')
 Invoke-Expression $text
}
`

func TestWindowsInstallerMakeAvailableInCurrentProcess(t *testing.T) {
	for _, scenario := range []string{"existing", "stale user PATH", "stale machine PATH", "installed user PATH", "installed machine PATH", "winget links", "missing winget", "winget failure", "missing make after install", "broken existing make", "broken installed make"} {
		t.Run(scenario, func(t *testing.T) {
			runWindowsInstallerFixture(t, `param($Installer, $Fixture, $Scenario)
`+windowsInstallerFixtureFunctions+`
$script:fixtureUserPath = ''; $script:fixtureMachinePath = ''
$env:LOCALAPPDATA = Join-Path $Fixture 'local app data'
$sessionDir = Join-Path $Fixture 'session-only Go'
$makeDir = Join-Path $Fixture 'GNU Make with spaces'
$links = Join-Path $env:LOCALAPPDATA 'Microsoft/WinGet/Links'
$env:Path = $sessionDir
$script:wingetCalls = 0; $script:makeCalls = 0; $script:persisted = @()
$script:availableDir = $makeDir
if ($Scenario -eq 'existing' -or $Scenario -eq 'broken existing make') { $env:Path += ";$makeDir" }
if ($Scenario -eq 'stale user PATH') { $script:fixtureUserPath = $makeDir }
if ($Scenario -eq 'stale machine PATH') { $script:fixtureMachinePath = $makeDir }
function Get-Command {
 param($Name, $ErrorAction)
 if ($Name -eq 'winget') {
  if ($Scenario -ne 'missing winget') { return [pscustomobject]@{Source='winget.exe'} }
  return $null
 }
 if ($Name -ne 'make') { throw "Unexpected command lookup $Name" }
 if (($env:Path -split ';') -contains $script:availableDir) { return [pscustomobject]@{Source=(Join-Path $script:availableDir 'make.exe')} }
 return $null
}
function make {
 if (($args -join ' ') -ne '--version') { throw 'Expected make --version' }
 $script:makeCalls++
 $global:LASTEXITCODE = if ($Scenario -like 'broken *') { 1 } else { 0 }
 'GNU Make 4.4.1'
}
function winget {
 $script:wingetCalls++
 if (($args -join ' ') -ne 'install --id ezwinports.make -e --scope user --accept-source-agreements --accept-package-agreements') { throw "Unexpected winget arguments: $args" }
 $global:LASTEXITCODE = 0
 switch ($Scenario) {
  'winget failure' { $global:LASTEXITCODE = 23 }
  'installed user PATH' { $script:fixtureUserPath = $makeDir }
  'installed machine PATH' { $script:fixtureMachinePath = $makeDir }
  'winget links' {
   New-Item -ItemType Directory -Force -Path $links | Out-Null
   $script:availableDir = $links
  }
  'broken installed make' { $script:fixtureUserPath = $makeDir }
 }
}
function Add-UserPathEntry { param($Entry) $script:persisted += $Entry }
$expectedFailure = switch ($Scenario) {
 'missing winget' { 'winget not found*' }
 'winget failure' { '*winget failed (exit code 23)*' }
 'missing make after install' { '*make.exe is unavailable*' }
 'broken existing make' { '*cannot run*' }
 'broken installed make' { '*cannot run*' }
 default { '' }
}
$caught = $false
try { Ensure-Make }
catch {
 $caught = $true
 if (-not $expectedFailure -or $_.Exception.Message -notlike $expectedFailure) { throw }
}
if ($expectedFailure -and -not $caught) { throw 'Expected failure was accepted' }
$expectedWinget = if ($Scenario -in @('existing', 'stale user PATH', 'stale machine PATH', 'broken existing make', 'missing winget')) { 0 } else { 1 }
if ($script:wingetCalls -ne $expectedWinget) { throw "Unexpected install attempts: $script:wingetCalls" }
if (-not (($env:Path -split ';') -contains $sessionDir)) { throw 'Session-only Go PATH was lost' }
if (-not $expectedFailure -and $script:makeCalls -ne 1) { throw 'make was not verified' }
if ($Scenario -eq 'winget links' -and $script:persisted -notcontains $links) { throw 'WinGet Links was not persisted' }
`, scenario)
		})
	}
}

func TestWindowsInstallerPathRefreshPreservesAndDeduplicatesEntries(t *testing.T) {
	runWindowsInstallerFixture(t, `param($Installer, $Fixture)
`+windowsInstallerFixtureFunctions+`
$env:SOLOMON_PATH_FIXTURE = Join-Path $Fixture 'expanded directory'
$env:Path = 'C:\session Go;C:\Shared\; ;C:\case'
$script:fixtureUserPath = 'c:\SHARED;%SOLOMON_PATH_FIXTURE%;C:\user'
$script:fixtureMachinePath = 'C:\CASE\;C:\machine'
Update-InstallerProcessPath
$expected = "C:\session Go;C:\Shared\;C:\case;$env:SOLOMON_PATH_FIXTURE;C:\user;C:\machine"
if ($env:Path -cne $expected) { throw "Unexpected refreshed PATH: $env:Path" }
Update-InstallerProcessPath
if ($env:Path -cne $expected) { throw 'PATH refresh is not idempotent' }
$env:Path = ''; $script:fixtureUserPath = ''; $script:fixtureMachinePath = ''
Update-InstallerProcessPath
if ($env:Path) { throw 'Empty PATH produced bogus entries' }
`)
}

func TestWindowsInstallerExecutionPolicyIsProcessOnly(t *testing.T) {
	for _, name := range []string{"README.md", "docs/user-guide/installation.md"} {
		data, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.ReplaceAll(string(data), "\r\n", "\n"), "Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force\nirm ") {
			t.Fatalf("%s: missing process bypass before online installer", name)
		}
	}
	runWindowsInstallerFixture(t, `param($Installer, $Fixture)
$ErrorActionPreference = 'Stop'
$tokens = $null; $errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($Installer, [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
$policyCommands = @($ast.FindAll({param($node) $node -is [System.Management.Automation.Language.CommandAst] -and $node.GetCommandName() -eq 'Set-ExecutionPolicy'}, $true))
if ($policyCommands.Count -ne 1) { throw 'Expected exactly one policy command' }
$script:policyCalls = 0
function Set-ExecutionPolicy {
 param($Scope, $ExecutionPolicy, [switch]$Force, $ErrorAction)
 if ($Scope -ne 'Process' -or $ExecutionPolicy -ne 'Bypass' -or -not $Force -or $ErrorAction -ne 'Stop') { throw 'Policy bypass must be process-only and failures must stop installation' }
 $script:policyCalls++
}
Invoke-Expression $policyCommands[0].Extent.Text
if ($script:policyCalls -ne 1) { throw 'Bypass was not invoked' }
# With Restricted, execute the online bootstrap (iex is permitted) in this isolated
# test process and verify the real policy changes only at Process scope on Windows.
Remove-Item Function:Set-ExecutionPolicy
if ([Environment]::OSVersion.Platform -eq [PlatformID]::Win32NT) {
 # Run startup before querying policy: inherited PowerShell 7 module paths can
 # otherwise break Windows PowerShell's own Security module in this fixture.
 $bootstrap = @($ast.EndBlock.Statements | Where-Object { $_ -isnot [System.Management.Automation.Language.FunctionDefinitionAst] -and $_.Extent.StartOffset -lt $policyCommands[0].Extent.StartOffset })
 foreach ($statement in $bootstrap) { Invoke-Expression $statement.Extent.Text }
 if ($PSVersionTable.PSEdition -eq 'Desktop' -and ($env:PSModulePath -split ';')[0] -ne (Join-Path $PSHOME 'Modules')) { throw 'Host modules must precede inherited modules' }
 $user = Get-ExecutionPolicy -Scope CurrentUser
 $machine = Get-ExecutionPolicy -Scope LocalMachine
 Set-ExecutionPolicy -Scope Process -ExecutionPolicy Restricted -Force
 foreach ($statement in $bootstrap) { Invoke-Expression $statement.Extent.Text }
 if ((Get-ExecutionPolicy -Scope Process) -ne 'Bypass') { throw 'Restricted process was not bypassed' }
 if ((Get-ExecutionPolicy -Scope CurrentUser) -ne $user -or (Get-ExecutionPolicy -Scope LocalMachine) -ne $machine) { throw 'Persistent policy changed' }
}
`)
}

func TestWindowsInstallerMakeRealCommandDiscovery(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Real command discovery requires Windows")
	}
	runWindowsInstallerFixture(t, `param($Installer, $Fixture)
`+windowsInstallerFixtureFunctions+`
$script:fixtureUserPath = ''; $script:fixtureMachinePath = ''
$env:LOCALAPPDATA = Join-Path $Fixture 'local app data'
$env:Path = Join-Path $Fixture 'session Go'
$links = Join-Path $env:LOCALAPPDATA 'Microsoft/WinGet/Links'
function Add-UserPathEntry { param($Entry) }
function winget {
 New-Item -ItemType Directory -Force -Path $links | Out-Null
 [IO.File]::WriteAllText((Join-Path $links 'make.cmd'), ('@echo off' + [Environment]::NewLine + 'if "%1"=="--version" (echo GNU Make 4.4.1 & exit /b 0)' + [Environment]::NewLine + 'exit /b 9' + [Environment]::NewLine))
 $global:LASTEXITCODE = 0
}
Ensure-Make
if (-not (Get-Command make -ErrorAction Stop)) { throw 'Real make command discovery failed' }
`)
}
