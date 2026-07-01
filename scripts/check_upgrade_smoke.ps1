#Requires -Version 5.1
$ErrorActionPreference = 'Stop'

if (-not $env:RELEASE_TAG) { throw 'RELEASE_TAG is required' }
if (-not $env:GITHUB_REPOSITORY) { throw 'GITHUB_REPOSITORY is required' }

$repo = $env:GITHUB_REPOSITORY
$legacyTag = if ($env:UPGRADE_SMOKE_LEGACY_TAG) { $env:UPGRADE_SMOKE_LEGACY_TAG } else { 'v2026.624.0' }
$fixedBaseline = if ($env:UPGRADE_SMOKE_FIXED_BASELINE) { $env:UPGRADE_SMOKE_FIXED_BASELINE } else { 'v2026.701.0' }
$smokeRoot = if ($env:UPGRADE_SMOKE_ROOT) { $env:UPGRADE_SMOKE_ROOT } else { Join-Path $env:TEMP ("solomon-upgrade-smoke-" + [guid]::NewGuid().ToString('N')) }
$strictErrorPatterns = @(
    'InvalidOperation',
    'RestartExe',
    'Cannot create a file when that file already exists'
)

function Get-PrevRelease {
    $headers = @{ 'User-Agent' = 'solomon-upgrade-smoke' }
    if ($env:GH_TOKEN) {
        $headers['Authorization'] = "Bearer $($env:GH_TOKEN)"
    }
    for ($attempt = 1; $attempt -le 5; $attempt++) {
        try {
            $releases = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases?per_page=2" -Headers $headers
            if ($releases.Count -ge 2) {
                return [string]$releases[1].tag_name
            }
        } catch {
            if ($attempt -ge 5) { throw }
            Write-Host "release lookup failed (attempt $attempt/5); retrying..."
            Start-Sleep -Seconds 2
        }
    }
    return ''
}

function Test-ReleaseExists {
    param([string]$Tag)
    try {
        Invoke-WebRequest -Uri "https://github.com/$repo/releases/tag/$Tag" -Method Head -UseBasicParsing | Out-Null
        return $true
    } catch {
        return $false
    }
}

function Test-VersionGe {
    param([string]$Left, [string]$Right)
    return ($Left.TrimStart('v') -ge $Right.TrimStart('v'))
}

function Test-HasCliMarker {
    param([string]$Exe)
    $bytes = [System.IO.File]::ReadAllBytes($Exe)
    $text = [System.Text.Encoding]::ASCII.GetString($bytes)
    return $text -match 'solomon-cli-upgrade-v1'
}

function Assert-LogStrict {
    param([string]$LogPath, [string]$FromTag)
    if (-not (Test-VersionGe $FromTag $fixedBaseline)) {
        return
    }
    if (-not (Test-Path $LogPath)) {
        throw "missing upgrade log: $LogPath"
    }
    $content = Get-Content -Path $LogPath -Raw
    foreach ($pattern in $strictErrorPatterns) {
        if ($content -match [regex]::Escape($pattern)) {
            throw "upgrade smoke: forbidden output for ${FromTag}: ${pattern}`n$content"
        }
    }
}

function Get-CaseDir {
    param([string]$FromTag)
    $safe = $FromTag -replace '[^A-Za-z0-9._-]', '_'
    return Join-Path $smokeRoot "$safe-cli"
}

function Get-DefaultBinDir {
    $gobin = (go env GOBIN).Trim()
    if ($gobin) {
        return $gobin
    }
    return Join-Path (go env GOPATH) 'bin'
}

function Install-Release {
    param([string]$FromTag)
    Remove-Item Env:GOBIN -ErrorAction SilentlyContinue
    $binDir = Get-DefaultBinDir
    if (-not (Test-PathHasDir $binDir)) {
        $env:Path = "$binDir;" + $env:Path
    }
    $script = irm "https://raw.githubusercontent.com/$repo/main/scripts/install.ps1"
    $block = [scriptblock]::Create($script)
    & $block -Version $FromTag
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

function Get-ExePath {
    return Join-Path (Get-DefaultBinDir) 'solomon.exe'
}

function Write-UpgradeLog {
    param([string]$LogPath)
    if (-not (Test-Path $LogPath)) {
        return
    }
    Write-Host "---- upgrade log ($LogPath) ----"
    Get-Content -Path $LogPath -Raw
    Write-Host '---- end upgrade log ----'
}

function Wait-TargetVersion {
    param([string]$Exe, [string]$LogPath = '')
    $last = ''
    for ($attempt = 1; $attempt -le 90; $attempt++) {
        $ver = (& $Exe version 2>&1 | Out-String).Trim()
        $last = $ver
        if ($ver -like "*$($env:RELEASE_TAG)*") {
            Write-Host "Upgrade smoke OK ($($env:RELEASE_TAG)): $ver"
            return
        }
        Start-Sleep -Seconds 2
    }
    Write-UpgradeLog $LogPath
    throw "Upgrade smoke failed: expected version to include $($env:RELEASE_TAG), last=$last"
}

function Invoke-CliUpgrade {
    param([string]$Exe, [string]$LogPath)
    $p = Start-Process -FilePath $Exe -ArgumentList 'upgrade' -NoNewWindow -PassThru -RedirectStandardOutput $LogPath -RedirectStandardError "${LogPath}.err"
    Wait-TargetVersion $Exe $LogPath
    $p.WaitForExit()
    if (Test-Path "${LogPath}.err") {
        Add-Content -Path $LogPath -Value (Get-Content -Path "${LogPath}.err" -Raw)
    }
}

function Invoke-UpgradeSmokeCase {
    param([string]$FromTag)

    $logPath = "$(Get-CaseDir $FromTag).log"
    Write-Host "Upgrade smoke (cli): $FromTag -> $($env:RELEASE_TAG)"
    Install-Release $FromTag
    $exe = Get-ExePath
    $env:NO_COLOR = '1'

    $current = (& $exe version 2>&1 | Out-String).Trim()
    Write-Host "Installed source release: $current"
    if ($current -notlike "*$FromTag*") {
        throw "expected version to include $FromTag, got $current"
    }

    if (-not (Test-HasCliMarker $exe)) {
        Write-Host "Skipping CLI upgrade smoke for ${FromTag}: no solomon upgrade CLI"
        return
    }
    Invoke-CliUpgrade $exe $logPath
    Assert-LogStrict $logPath $FromTag
}

$prev = Get-PrevRelease
$sources = @()
if ($prev -and $prev -ne $env:RELEASE_TAG) {
    $sources += $prev
}
if ($legacyTag -ne $env:RELEASE_TAG -and $legacyTag -ne $prev -and (Test-ReleaseExists $legacyTag)) {
    $sources += $legacyTag
}
if ($sources.Count -eq 0) {
    Write-Host 'Skipping upgrade smoke: no source tags to test'
    exit 0
}

New-Item -ItemType Directory -Force -Path $smokeRoot | Out-Null
foreach ($fromTag in $sources) {
    if ($fromTag -eq $env:RELEASE_TAG) { continue }
    Invoke-UpgradeSmokeCase $fromTag
}
