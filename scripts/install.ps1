#Requires -Version 5.1
param(
    [string]$Version = $(if ($env:SOLOMON_VERSION) { $env:SOLOMON_VERSION } else { 'latest' })
)

$ErrorActionPreference = 'Stop'

$GoRequired = '1.25.0'
$GoRoot = Join-Path $env:USERPROFILE '.local\go'
$script:InstalledLocalGo = $false
$SolmonPkg = "github.com/SAPPHIR3-ROS3/Solomon/cmd/solomon@$Version"
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

function Ensure-Go {
    $goCmd = Get-Command go -ErrorAction SilentlyContinue
    if ($goCmd) {
        $ver = Get-GoSemVer
        if (Test-VersionGe -Have $ver -Want $GoRequired) {
            Write-Host "Go $ver OK (>= $GoRequired)"
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

function Add-UserPathEntry {
    param([string]$Entry)
    if ([string]::IsNullOrWhiteSpace($Entry)) { return }
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $parts = $userPath -split ';' | Where-Object { $_ -and $_.Trim() -ne '' }
    if ($parts -contains $Entry) { return }
    $newPath = if ($userPath) { "$userPath;$Entry" } else { $Entry }
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
    if ($env:Path -notlike "*$Entry*") {
        $env:Path = "$Entry;$env:Path"
    }
    Write-Host "Added to user PATH: $Entry"
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

function Setup-Shell {
    if ($script:InstalledLocalGo) {
        Add-UserPathEntry (Join-Path $GoRoot 'bin')
    }

    $gopathBin = Join-Path (go env GOPATH) 'bin'
    Add-UserPathEntry $gopathBin

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

    $block = @"
$Marker
`$goBin = Join-Path `$env:USERPROFILE '.local\go\bin'
if (Test-Path `$goBin) { `$env:Path = "`$goBin;" + `$env:Path }
`$gopathBin = Join-Path (go env GOPATH) 'bin'
if ((Test-Path `$gopathBin) -and (`$env:Path -notlike "*`$gopathBin*")) { `$env:Path += ";`$gopathBin" }
"@

    $content = Get-Content -Path $profile -Raw -ErrorAction SilentlyContinue
    if ($content -and $content.Contains($Marker)) {
        Write-Host "PowerShell profile already configured: $profile"
    }
    else {
        Add-Content -Path $profile -Value "`n$block"
        Write-Host "Updated PowerShell profile: $profile"
    }

    Write-Host 'Restart the terminal or run: . $PROFILE'
}

function Install-Solomon {
    Write-Host "Installing solomon ($Version)..."
    go install $SolmonPkg
    $bin = Join-Path (go env GOPATH) 'bin\solomon.exe'
    if (Test-Path $bin) {
        Write-Host "solomon installed: $bin"
        & $bin version 2>$null
    }
    else {
        Write-Host "solomon binary expected at $bin"
    }
}

Ensure-Go
Ensure-Make
Setup-Shell
Install-Solomon
Write-Host 'Done.'
