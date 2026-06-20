#Requires -Version 5.1
param(
    [string]$Version = $(if ($env:SOLOMON_VERSION) { $env:SOLOMON_VERSION } else { 'latest' }),
    [switch]$SetupPathOnly
)

$ErrorActionPreference = 'Stop'

$GoRequired = '1.25.0'
$GoRoot = Join-Path $env:USERPROFILE '.local\go'
$script:InstalledLocalGo = $false
$GithubReleasesLatest = 'https://api.github.com/repos/SAPPHIR3-ROS3/Solomon/releases/latest'
$Marker = '# solomon-installer'

function Get-GoSemVer {
    if ((go version 2>$null) -match 'go version go(\S+)') {
        return ($Matches[1] -replace '-.*$', '')
    }
    return ''
}

function Test-VersionGe {
    param([string]$Have, [string]$Want)
    $h = [version]($Have -replace '^go', '')
    $w = [version]$Want
    return $h -ge $w
}

function Get-GoArch {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        'AMD64' { return 'amd64' }
        'ARM64' { return 'arm64' }
        default {
            throw "unsupported architecture: $($env:PROCESSOR_ARCHITECTURE)"
        }
    }
}

function Install-GoWindows {
    $arch = Get-GoArch
    $zip = "go$GoRequired.windows-$arch.zip"
    $url = "https://go.dev/dl/$zip"
    $parent = Split-Path $GoRoot -Parent
    New-Item -ItemType Directory -Force -Path $parent | Out-Null
    $tmp = Join-Path $env:TEMP "solomon-go-$([guid]::NewGuid().ToString('n'))"
    New-Item -ItemType Directory -Force -Path $tmp | Out-Null
    try {
        Write-Host "Downloading Go $GoRequired (windows-$arch)..."
        $zipPath = Join-Path $tmp $zip
        Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing
        if (Test-Path $GoRoot) {
            Remove-Item -Recurse -Force $GoRoot
        }
        Expand-Archive -Path $zipPath -DestinationPath $parent -Force
    }
    finally {
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }
    $env:Path = "$(Join-Path $GoRoot 'bin');$env:Path"
    $script:InstalledLocalGo = $true
}

function Get-LatestReleaseTag {
    $headers = @{ 'User-Agent' = 'solomon-installer' }
    $resp = Invoke-RestMethod -Uri $GithubReleasesLatest -Headers $headers -UseBasicParsing
    $tag = [string]$resp.tag_name
    if ([string]::IsNullOrWhiteSpace($tag)) {
        throw 'failed to resolve latest GitHub release tag'
    }
    return $tag.Trim()
}

function Resolve-InstallVersion {
    if ($Version -ne 'latest') {
        return
    }
    $script:Version = Get-LatestReleaseTag
    Write-Host "Latest release: $Version"
}

function Install-ReleaseAsset {
    $arch = Get-GoArch
    $asset = "solomon-$Version-windows-$arch.exe"
    $url = "https://github.com/SAPPHIR3-ROS3/Solomon/releases/download/$Version/$asset"
    $binDir = Get-GoInstallBinDir
    $target = Join-Path $binDir 'solomon.exe'
    New-Item -ItemType Directory -Force -Path $binDir | Out-Null
    Write-Host "Downloading Solomon release asset $asset..."
    $maxAttempts = 15
    $tmp = Join-Path $env:TEMP ("solomon-" + [guid]::NewGuid().ToString('n'))
    for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
        try {
            Invoke-WebRequest -Uri $url -OutFile $tmp -UseBasicParsing
            break
        } catch {
            if ($attempt -ge $maxAttempts) {
                throw "Failed to download $asset after $maxAttempts attempts"
            }
            Write-Host "Download failed (attempt $attempt/$maxAttempts), retrying..."
            Start-Sleep -Seconds 2
        }
    }
    $checksumsUrl = "https://github.com/SAPPHIR3-ROS3/Solomon/releases/download/$Version/checksums.txt"
    $checksumsPath = Join-Path $env:TEMP ("solomon-checksums-" + [guid]::NewGuid().ToString('n'))
    try {
        Invoke-WebRequest -Uri $checksumsUrl -OutFile $checksumsPath -UseBasicParsing
        $expected = $null
        Get-Content $checksumsPath | ForEach-Object {
            if ($_ -match '^(\S+)\s+\*?(.+)$') {
                if ($Matches[2].Trim() -eq $asset) { $expected = $Matches[1].ToLower() }
            }
        }
        if (-not $expected) {
            throw "checksums: no entry for $asset in checksums.txt"
        }
        $actual = (Get-FileHash $tmp -Algorithm SHA256).Hash.ToLower()
        if ($expected -ne $actual) {
            throw "checksum mismatch for $asset (expected $expected, got $actual)"
        }
    } catch [System.Net.WebException] {
        if ($_.Exception.Response -and $_.Exception.Response.StatusCode.value__ -eq 404) {
            Write-Warning "no checksums.txt for $Version; skipping integrity check"
        } else {
            throw
        }
    } finally {
        Remove-Item -Force $checksumsPath -ErrorAction SilentlyContinue
    }
    Move-Item -Force $tmp $target
}

function Ensure-Go {
    $goCmd = Get-Command go -ErrorAction SilentlyContinue
    if ($goCmd) {
        $ver = Get-GoSemVer
        if (Test-VersionGe -Have $ver -Want $GoRequired) {
            Write-Host "Go $ver OK (>= $GoRequired)"
            Ensure-GoBinInPath
            return
        }
        Write-Host "Go $ver is older than $GoRequired; upgrading..."
    }
    else {
        Write-Host "Go not found; installing $GoRequired..."
    }
    Install-GoWindows
    $ver = Get-GoSemVer
    if (-not (Test-VersionGe -Have $ver -Want $GoRequired)) {
        throw "Go install failed (got $ver)"
    }
    Write-Host "Go $ver ready"
    Ensure-GoBinInPath
}

function Ensure-Node {
    $nodeCmd = Get-Command node -ErrorAction SilentlyContinue
    if ($nodeCmd) {
        $ver = (node --version 2>$null).Trim()
        Write-Host "Node $ver OK"
        return
    }

    Write-Host 'Node not found; installing LTS via winget...'
    $winget = Get-Command winget -ErrorAction SilentlyContinue
    if (-not $winget) {
        throw 'winget not found; install Node.js LTS from https://nodejs.org/en/download/'
    }

    & winget install --id OpenJS.NodeJS.LTS -e --accept-source-agreements --accept-package-agreements

    $nodeDirs = @(
        (Join-Path $env:ProgramFiles 'nodejs')
        (Join-Path ${env:ProgramFiles(x86)} 'nodejs')
    )
    foreach ($dir in $nodeDirs) {
        if (Test-Path $dir) {
            Add-UserPathEntry $dir
            break
        }
    }

    if (-not (Get-Command node -ErrorAction SilentlyContinue)) {
        foreach ($dir in $nodeDirs) {
            $nodeExe = Join-Path $dir 'node.exe'
            if (Test-Path $nodeExe) {
                $env:Path = "$dir;$env:Path"
                break
            }
        }
    }

    if (-not (Get-Command node -ErrorAction SilentlyContinue)) {
        throw 'Node install failed; restart the terminal or install manually from https://nodejs.org/'
    }

    Write-Host "Node $((node --version).Trim()) ready"
}

function Get-GoInstallBinDir {
    $gobin = (go env GOBIN).Trim()
    if ($gobin) {
        return $gobin
    }
    return Join-Path (go env GOPATH) 'bin'
}

function Ensure-GoBinInPath {
    $binDir = Get-GoInstallBinDir
    New-Item -ItemType Directory -Force -Path $binDir | Out-Null
    Add-UserPathEntry $binDir
    $sessionParts = $env:Path -split ';' | Where-Object {
        $_ -and $_.Trim() -ne '' -and $_.Trim().TrimEnd('\') -ine $binDir
    }
    $env:Path = if ($sessionParts.Count -gt 0) { "$binDir;" + ($sessionParts -join ';') } else { $binDir }
}

function Add-UserPathEntry {
    param([string]$Entry)
    if ([string]::IsNullOrWhiteSpace($Entry)) { return }
    $Entry = $Entry.Trim().TrimEnd('\')
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $userParts = @()
    if ($userPath) {
        $userParts = $userPath -split ';' | Where-Object {
            $_ -and $_.Trim() -ne '' -and $_.Trim().TrimEnd('\') -ine $Entry
        }
    }
    $newUserPath = if ($userParts.Count -gt 0) { "$Entry;" + ($userParts -join ';') } else { $Entry }
    if ($userPath -ne $newUserPath) {
        [Environment]::SetEnvironmentVariable('Path', $newUserPath, 'User')
        Write-Host "Ensured user PATH includes: $Entry"
    }
}

function Ensure-Make {
    $makeCmd = Get-Command make -ErrorAction SilentlyContinue
    if ($makeCmd) {
        Write-Host "make OK ($($makeCmd.Source))"
        return
    }

    Write-Host 'make not found; installing ezwinports.make via winget...'
    $winget = Get-Command winget -ErrorAction SilentlyContinue
    if (-not $winget) {
        throw 'winget not found; install GNU Make manually or install App Installer from the Microsoft Store'
    }

    & winget install --id ezwinports.make -e --accept-source-agreements --accept-package-agreements

    $wingetLinks = Join-Path $env:LOCALAPPDATA 'Microsoft\WinGet\Links'
    if (Test-Path $wingetLinks) {
        Add-UserPathEntry $wingetLinks
    }

    if (-not (Get-Command make -ErrorAction SilentlyContinue) -and (Test-Path $wingetLinks)) {
        $env:Path = "$wingetLinks;$env:Path"
    }

    $makeCmd = Get-Command make -ErrorAction SilentlyContinue
    if (-not $makeCmd) {
        throw 'make install failed; restart the terminal or verify ezwinports.make is available in PATH'
    }

    Write-Host "make ready ($($makeCmd.Source))"
}

function Get-GoInstallBinProfileBlock {
    return @"
$Marker
`$goBin = Join-Path `$env:USERPROFILE '.local\go\bin'
if ((Test-Path `$goBin) -and (`$env:Path -notlike "*`$goBin*")) { `$env:Path = "`$goBin;" + `$env:Path }
`$goInstallBin = (go env GOBIN).Trim()
if (-not `$goInstallBin) {
    `$goInstallBin = Join-Path (go env GOPATH) 'bin'
}
if (Test-Path `$goInstallBin) {
    `$sessionParts = `$env:Path -split ';' | Where-Object {
        `$_ -and `$_.Trim() -ne '' -and `$_.Trim().TrimEnd('\') -ine `$goInstallBin
    }
    `$env:Path = if (`$sessionParts.Count -gt 0) { "`$goInstallBin;" + (`$sessionParts -join ';') } else { `$goInstallBin }
}
"@
}

function Install-GoInstallBinProfileBlock {
    param([string]$ProfilePath)

    $block = Get-GoInstallBinProfileBlock
    $content = Get-Content -Path $ProfilePath -Raw -ErrorAction SilentlyContinue
    if ([string]::IsNullOrEmpty($content)) {
        Set-Content -Path $ProfilePath -Value $block.TrimStart("`n") -NoNewline
        Write-Host "Updated PowerShell profile: $ProfilePath"
        return
    }
    if ($content.Contains($Marker)) {
        $pattern = '(?s)\r?\n' + [regex]::Escape($Marker) + '.*'
        $updated = [regex]::Replace($content, $pattern, "`n$($block.Trim())", 1)
        Set-Content -Path $ProfilePath -Value $updated.TrimEnd() -NoNewline
        Write-Host "Updated PowerShell profile: $ProfilePath"
        return
    }
    Add-Content -Path $ProfilePath -Value "`n$block"
    Write-Host "Updated PowerShell profile: $ProfilePath"
}

function Setup-Shell {
    if ($script:InstalledLocalGo) {
        Add-UserPathEntry (Join-Path $GoRoot 'bin')
        $goBin = Join-Path $GoRoot 'bin'
        $sessionParts = $env:Path -split ';' | Where-Object {
            $_ -and $_.Trim() -ne '' -and $_.Trim().TrimEnd('\') -ine $goBin
        }
        $env:Path = if ($sessionParts.Count -gt 0) { "$goBin;" + ($sessionParts -join ';') } else { $goBin }
    }

    Ensure-GoBinInPath

    $profile = $PROFILE.CurrentUserAllHosts
    if (-not $profile) {
        $profile = Join-Path ([Environment]::GetFolderPath('MyDocuments')) 'PowerShell\profile.ps1'
    }
    $profileDir = Split-Path $profile -Parent
    if (-not (Test-Path $profileDir)) {
        New-Item -ItemType Directory -Force -Path $profileDir | Out-Null
    }
    if (-not (Test-Path $profile)) {
        New-Item -ItemType File -Force -Path $profile | Out-Null
    }

    Install-GoInstallBinProfileBlock -ProfilePath $profile

    Write-Host 'Restart the terminal or run: . $PROFILE'
}

function Install-Solomon {
    Resolve-InstallVersion
    Ensure-GoBinInPath
    Write-Host "Installing solomon ($Version)..."
    Install-ReleaseAsset
    $binDir = Get-GoInstallBinDir
    $bin = Join-Path $binDir 'solomon.exe'
    if (Test-Path $bin) {
        Write-Host "solomon installed: $bin"
        & $bin init 2>$null
        & $bin version 2>$null
        $cmd = Get-Command solomon -ErrorAction SilentlyContinue
        if ($cmd) {
            Write-Host "solomon on PATH: $($cmd.Source)"
        }
    }
    else {
        throw "solomon binary expected at $bin"
    }
}

if ($SetupPathOnly) {
    Ensure-Go
    Setup-Shell
    return
}

Ensure-Go
Ensure-Make
Setup-Shell
Install-Solomon
Write-Host 'Done.'
