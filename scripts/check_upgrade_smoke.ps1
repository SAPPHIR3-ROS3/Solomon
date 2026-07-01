#Requires -Version 5.1
$ErrorActionPreference = 'Stop'

if (-not $env:RELEASE_TAG) { throw 'RELEASE_TAG is required' }
if (-not $env:GITHUB_REPOSITORY) { throw 'GITHUB_REPOSITORY is required' }

$repo = $env:GITHUB_REPOSITORY
$headers = @{ 'User-Agent' = 'solomon-upgrade-smoke' }
if ($env:GH_TOKEN) {
    $headers['Authorization'] = "Bearer $($env:GH_TOKEN)"
}
$prev = ''
for ($attempt = 1; $attempt -le 5; $attempt++) {
    try {
        $releases = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases?per_page=2" -Headers $headers
        if ($releases.Count -ge 2) {
            $prev = [string]$releases[1].tag_name
        }
        if (-not [string]::IsNullOrWhiteSpace($prev)) {
            break
        }
    } catch {
        if ($attempt -ge 5) {
            throw
        }
        Write-Host "release lookup failed (attempt $attempt/5); retrying..."
        Start-Sleep -Seconds 2
    }
}
if ([string]::IsNullOrWhiteSpace($prev) -or $prev -eq $env:RELEASE_TAG) {
    Write-Host "Skipping upgrade smoke: no previous release to upgrade from (prev=$prev)"
    exit 0
}

Write-Host "Upgrade smoke: $prev -> $env:RELEASE_TAG"
$script = irm "https://raw.githubusercontent.com/$repo/main/scripts/install.ps1"
$block = [scriptblock]::Create($script)
& $block -Version $prev

$gobin = (go env GOBIN).Trim()
if ($gobin) {
    $binDir = $gobin
} else {
    $binDir = Join-Path (go env GOPATH) 'bin'
}
$exe = Join-Path $binDir 'solomon.exe'
$env:NO_COLOR = '1'

$current = (& $exe version 2>&1 | Out-String).Trim()
Write-Host "Installed previous release: $current"
if ($current -notlike "*$prev*") {
    throw "expected version to include $prev, got $current"
}
$bytes = [System.IO.File]::ReadAllBytes($exe)
$text = [System.Text.Encoding]::ASCII.GetString($bytes)
if ($text -notmatch 'solomon-cli-upgrade-v1') {
    Write-Host "Skipping upgrade smoke: $prev predates solomon upgrade CLI"
    exit 0
}

$p = Start-Process -FilePath $exe -ArgumentList 'upgrade' -NoNewWindow -PassThru -Wait
if ($p.ExitCode -ne 0) {
    throw "solomon upgrade exited with code $($p.ExitCode)"
}

for ($attempt = 1; $attempt -le 90; $attempt++) {
    $ver = (& $exe version 2>&1 | Out-String).Trim()
    if ($ver -like "*$($env:RELEASE_TAG)*") {
        Write-Host "Upgrade smoke OK: $ver"
        exit 0
    }
    Start-Sleep -Seconds 2
}

throw "Upgrade smoke failed: expected version to include $($env:RELEASE_TAG), last=$ver"
