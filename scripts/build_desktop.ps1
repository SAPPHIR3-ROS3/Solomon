#Requires -Version 5.1
param(
    [switch]$Install,
    [string]$BinDir,
    [string]$Version = 'dev'
)
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent

Push-Location (Join-Path $root 'gui\desktop')
try {
    $wailsVersion = (go list -m -f '{{.Version}}' github.com/wailsapp/wails/v2).Trim()
    Write-Host 'Compiling Solomon desktop...'
    $buildLog = Join-Path $env:TEMP ("solomon-desktop-build-$([guid]::NewGuid().ToString('n')).log")
    try {
        $previousErrorPreference = $ErrorActionPreference
        try {
            $ErrorActionPreference = 'Continue'
            & go run "github.com/wailsapp/wails/v2/cmd/wails@$wailsVersion" build -skipbindings -nosyncgomod -m *> $buildLog
            $buildExitCode = $LASTEXITCODE
        }
        finally { $ErrorActionPreference = $previousErrorPreference }
        if ($buildExitCode -ne 0) {
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
        if (-not $BinDir) { $BinDir = Join-Path (((go env GOPATH) -split ';')[0]) 'bin' }
    }
    $BinDir = [IO.Path]::GetFullPath($BinDir)
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    Push-Location $root
    $cliBuild = Join-Path $BinDir (".solomon-cli-$([guid]::NewGuid().ToString('n')).tmp")
    try {
        $commit = (git rev-parse HEAD).Trim()
        $commitTime = (git show -s --format=%cI HEAD).Trim()
        $sourceTree = (go run ./scripts/source_identity).Trim()
        if ($LASTEXITCODE -ne 0) { throw 'Source identity calculation failed' }
        $commitTree = (git rev-parse 'HEAD^{tree}').Trim()
        $metadata = "-s -w -X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.version=$Version -X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.commit=$commit -X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.commitTime=$commitTime -X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.sourceTree=$sourceTree -X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.commitTree=$commitTree"
        & go build -trimpath -ldflags $metadata -o $cliBuild ./cmd/solomon
        if ($LASTEXITCODE -ne 0) { throw 'CLI build failed' }
        & go run ./scripts/install_local $cliBuild (Join-Path $root 'gui\desktop\build\bin\solomon-desktop.exe') (Join-Path $BinDir 'solomon.exe') $Version
        if ($LASTEXITCODE -ne 0) { throw 'Coordinated desktop installation failed; inspect Solomon server logs' }
    }
    finally {
        Remove-Item -LiteralPath $cliBuild -Force -ErrorAction SilentlyContinue
        Pop-Location
    }
}
