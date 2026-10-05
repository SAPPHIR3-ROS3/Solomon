#Requires -Version 5.1
param(
    [switch]$Install,
    [string]$BinDir,
    [string]$Version = 'dev'
)
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent

function Install-BuiltExecutable {
    param([string]$Source, [string]$Target)
    $staged = "$Target.$([guid]::NewGuid().ToString('n')).tmp"
    $backup = "$Target.$([guid]::NewGuid().ToString('n')).bak"
    Copy-Item -LiteralPath $Source -Destination $staged -Force
    try {
        if (Test-Path -LiteralPath $Target) { Move-Item -LiteralPath $Target -Destination $backup }
        try { Move-Item -LiteralPath $staged -Destination $Target }
        catch {
            if (Test-Path -LiteralPath $backup) { Move-Item -LiteralPath $backup -Destination $Target }
            throw
        }
    }
    finally {
        Remove-Item -LiteralPath $staged, $backup -Force -ErrorAction SilentlyContinue
    }
}

Push-Location (Join-Path $root 'gui\desktop')
try {
    $wailsVersion = (go list -m -f '{{.Version}}' github.com/wailsapp/wails/v2).Trim()
    Write-Host 'Compiling Solomon desktop...'
    $buildLog = Join-Path $env:TEMP ("solomon-desktop-build-$([guid]::NewGuid().ToString('n')).log")
    try {
        & go run "github.com/wailsapp/wails/v2/cmd/wails@$wailsVersion" build -skipbindings -nosyncgomod -m *> $buildLog
        if ($LASTEXITCODE -ne 0) {
            Get-Content -LiteralPath $buildLog | Out-Host
            throw 'Desktop build failed'
        }
        Write-Host 'Solomon desktop compiled.'
    }
    finally { Remove-Item -LiteralPath $buildLog -Force -ErrorAction SilentlyContinue }
}
finally { Pop-Location }

if ($Install) {
    if (-not $BinDir) {
        $BinDir = (go env GOBIN).Trim()
        if (-not $BinDir) { $BinDir = Join-Path (go env GOPATH) 'bin' }
    }
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    Push-Location $root
    $cliBuild = Join-Path $env:TEMP ("solomon-cli-$([guid]::NewGuid().ToString('n')).exe")
    try {
        $commit = (git rev-parse HEAD).Trim()
        $commitTime = (git show -s --format=%cI HEAD).Trim()
        $metadata = "-X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.version=$Version -X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.commit=$commit -X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.commitTime=$commitTime"
        & go build -trimpath -ldflags $metadata -o $cliBuild ./cmd/solomon
        if ($LASTEXITCODE -ne 0) { throw 'CLI build failed' }
        Install-BuiltExecutable -Source $cliBuild -Target (Join-Path $BinDir 'solomon.exe')
    }
    finally {
        Remove-Item -LiteralPath $cliBuild -Force -ErrorAction SilentlyContinue
        Pop-Location
    }
    Install-BuiltExecutable -Source (Join-Path $root 'gui\desktop\build\bin\solomon-desktop.exe') -Target (Join-Path $BinDir 'solomon-desktop.exe')
    & (Join-Path $BinDir 'solomon.exe') init
    if ($LASTEXITCODE -ne 0) { throw 'Desktop registration failed' }
}
