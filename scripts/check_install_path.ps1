#Requires -Version 5.1
$ErrorActionPreference = 'Stop'

$root = Split-Path $PSScriptRoot -Parent
$marker = '# solomon-installer'

function Get-GoInstallBinDir {
    $gobin = (go env GOBIN).Trim()
    if ($gobin) {
        return $gobin
    }
    return Join-Path (go env GOPATH) 'bin'
}

function Remove-PathEntry {
    param([string]$Entry)
    $Entry = $Entry.Trim().TrimEnd('\')
    $env:Path = ($env:Path -split ';' | Where-Object {
        $_ -and $_.Trim() -ne '' -and $_.Trim().TrimEnd('\') -ine $Entry
    }) -join ';'
}

function Test-PathHasDir {
    param([string]$Dir)
    $Dir = $Dir.Trim().TrimEnd('\')
    foreach ($part in ($env:Path -split ';')) {
        if ($part -and $part.Trim().TrimEnd('\') -ieq $Dir) {
            return $true
        }
    }
    return $false
}

function Test-InstallPathCase {
    param(
        [string]$Label,
        [string]$HomeDir,
        [scriptblock]$BeforeSetup
    )

    Write-Host "Checking install PATH setup ($Label)..."
    Remove-Item -Recurse -Force $HomeDir -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force -Path $HomeDir | Out-Null
    $env:USERPROFILE = $HomeDir
    $env:HOME = $HomeDir
    $env:Path = "$(go env GOROOT)\bin;$env:Path"
    Remove-PathEntry (Join-Path (go env GOPATH) 'bin')
    $gobin = (go env GOBIN).Trim()
    if ($gobin) {
        Remove-PathEntry $gobin
    }
    & $BeforeSetup
    & (Join-Path $PSScriptRoot 'install.ps1') -SetupPathOnly

    $binDir = Get-GoInstallBinDir
    if (-not (Test-PathHasDir $binDir)) {
        throw "Go install bin not in session PATH: $binDir"
    }
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($userPath -notlike "*$binDir*") {
        throw "Go install bin not in user PATH: $binDir"
    }

    $profile = $PROFILE.CurrentUserAllHosts
    if (-not $profile) {
        $profile = Join-Path ([Environment]::GetFolderPath('MyDocuments')) 'PowerShell\profile.ps1'
    }
    if (-not (Test-Path $profile)) {
        throw "PowerShell profile not found: $profile"
    }
    $content = Get-Content -Path $profile -Raw
    if (-not $content.Contains($marker)) {
        throw "missing installer marker in PowerShell profile"
    }
    if (-not $content.Contains('go env GOBIN')) {
        throw 'missing GOBIN handling in PowerShell profile'
    }
}

$gopathHome = Join-Path $env:RUNNER_TEMP 'solomon-path-check-gopath'
Test-InstallPathCase -Label 'GOPATH/bin' -HomeDir $gopathHome -BeforeSetup {
    Remove-Item Env:GOBIN -ErrorAction SilentlyContinue
}

$gobinHome = Join-Path $env:RUNNER_TEMP 'solomon-path-check-gobin'
Test-InstallPathCase -Label 'GOBIN' -HomeDir $gobinHome -BeforeSetup {
    $custom = Join-Path $gobinHome 'custom-go-bin'
    New-Item -ItemType Directory -Force -Path $custom | Out-Null
    $env:GOBIN = $custom
}

Write-Host 'install PATH setup OK'
