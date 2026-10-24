package test

import (
	"path/filepath"
	"testing"
)

func TestWindowsUpgradeSmokeWaitsForDaemonAndPriorProcesses(t *testing.T) {
	runWindowsInstallerFixture(t, `param($Installer, $Fixture, $Smoke)
$ErrorActionPreference = 'Stop'
$tokens = $null; $errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($Smoke, [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
foreach ($definition in $ast.FindAll({param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst]}, $true)) {
 Invoke-Expression $definition.Extent.Text
}
$env:RELEASE_TAG = 'v2026.1024.1'
$script:daemonOutput = ("Installing Solomon Desktop ($env:RELEASE_TAG)..." + [Environment]::NewLine + "daemon unavailable")
$script:nativeFailure = $false
$script:stopCalls = 0
$script:sleeps = 0
function Start-Sleep { param($Seconds, $Milliseconds); $script:sleeps++ }
function FixtureSolomon {
 param($Command, $Argument)
 $global:LASTEXITCODE = 0
 if ($script:nativeFailure) { $global:LASTEXITCODE = 1 }
 if ($Command -eq 'server') { $script:stopCalls++; return }
 if ($Argument -eq '--binary') { return $env:RELEASE_TAG }
 return $script:daemonOutput
}
foreach ($output in @($script:daemonOutput, "$env:RELEASE_TAG-dev", "$($env:RELEASE_TAG)0")) {
 if (Test-ReleaseVersionOutput $output $env:RELEASE_TAG) { throw "Accepted false version: $output" }
}
$colored = "$([char]27)[32m$env:RELEASE_TAG$([char]27)[0m"
if (-not (Test-ReleaseVersionOutput $colored $env:RELEASE_TAG)) { throw 'Rejected colored version line' }
try { Wait-TargetVersion FixtureSolomon '' 1; throw 'Accepted setup log as daemon version' }
catch { if ($_ -notmatch 'Upgrade smoke failed') { throw } }
$script:daemonOutput = ("===SYSTEM===" + [Environment]::NewLine + $env:RELEASE_TAG + [Environment]::NewLine + "===SYSTEM===")
Wait-TargetVersion FixtureSolomon '' 1
$script:nativeFailure = $true
try { Wait-TargetVersion FixtureSolomon '' 1; throw 'Accepted nonzero native exit status' }
catch { if ($_ -notmatch 'Upgrade smoke failed') { throw } }
$script:nativeFailure = $false

# Verify process selection against unrelated executables and other homes.
$env:SOLOMON_HOME = Join-Path $Fixture 'home'
$exe = Join-Path $Fixture 'solomon.exe'
$desktop = Join-Path $Fixture 'solomon-desktop.exe'
$coordinator = Join-Path $env:SOLOMON_HOME 'logs/update/coordinator-fixture.exe'
$script:processes = @(
 [pscustomobject]@{ProcessId=1; ExecutablePath=$exe},
 [pscustomobject]@{ProcessId=2; ExecutablePath=$desktop},
 [pscustomobject]@{ProcessId=3; ExecutablePath=$coordinator},
 [pscustomobject]@{ProcessId=4; ExecutablePath=(Join-Path $Fixture 'unrelated/solomon.exe')},
 [pscustomobject]@{ProcessId=5; ExecutablePath=(Join-Path $Fixture 'other-home/logs/update/coordinator-other.exe')}
)
function Get-CimInstance { param($ClassName, $ErrorAction); return $script:processes }
$selected = @(Get-SmokeInstallationProcesses $exe)
if (($selected.ProcessId -join ',') -ne '1,2,3') { throw "Incorrect process selection: $($selected.ProcessId)" }
$selected = @(Get-SmokeInstallationProcesses $exe -CoordinatorsOnly)
if (($selected.ProcessId -join ',') -ne '3') { throw 'Coordinator selection included unrelated processes' }

# A completed version probe must not allow a subsequent installer to race shutdown.
$script:coordinatorPolls = 0
$script:stopCalls = 0
$script:sleeps = 0
function Get-SmokeInstallationProcesses {
 param($Exe, [switch]$CoordinatorsOnly)
 if ($CoordinatorsOnly) {
  $script:coordinatorPolls++
  if ($script:coordinatorPolls -lt 3) { return [pscustomobject]@{ProcessId=3} }
 } elseif ($script:stopCalls -lt 2) { return [pscustomobject]@{ProcessId=1} }
}
Stop-SmokeInstallation FixtureSolomon 5
if ($script:coordinatorPolls -ne 3 -or $script:stopCalls -ne 2 -or $script:sleeps -ne 3) {
 throw 'Did not wait for coordinator completion and final daemon shutdown'
}
function Get-SmokeInstallationProcesses { param($Exe, [switch]$CoordinatorsOnly); return [pscustomobject]@{ProcessId=99} }
try { Wait-UpgradeCoordinators FixtureSolomon 1; throw 'Accepted stuck coordinator' }
catch { if ($_ -notmatch 'coordinator did not finish') { throw } }
function Get-SmokeInstallationProcesses { param($Exe, [switch]$CoordinatorsOnly); if (-not $CoordinatorsOnly) { return [pscustomobject]@{ProcessId=99} } }
try { Stop-SmokeInstallation FixtureSolomon 1; throw 'Accepted running installation' }
catch { if ($_ -notmatch 'still running') { throw } }
`, filepath.Join(repoRoot(t), "scripts", "check_upgrade_smoke.ps1"))
}
