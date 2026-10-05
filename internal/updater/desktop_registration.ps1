#Requires -Version 5.1
param([string]$CLIPath, [string]$DesktopPath, [string]$Version)
$ErrorActionPreference = 'Stop'
$folder = Join-Path ([Environment]::GetFolderPath('Programs')) 'Solomon'
New-Item -ItemType Directory -Force -Path $folder | Out-Null
$shortcut = (New-Object -ComObject WScript.Shell).CreateShortcut((Join-Path $folder 'Solomon.lnk'))
$shortcut.TargetPath = $DesktopPath
$shortcut.WorkingDirectory = $env:USERPROFILE
$shortcut.IconLocation = "$DesktopPath,0"
$shortcut.Description = 'Solomon AI coding assistant'
$shortcut.Save()
$binDir = Split-Path -Parent $CLIPath
$uninstallPath = Join-Path $binDir 'uninstall-solomon.ps1'
$escapedCLI = $CLIPath.Replace("'", "''")
$escapedDesktop = $DesktopPath.Replace("'", "''")
$escapedFolder = $folder.Replace("'", "''")
$uninstall = @"
`$ErrorActionPreference = 'Stop'
& '$escapedCLI' server stop
Get-CimInstance Win32_Process | Where-Object {
    `$_.ExecutablePath -eq '$escapedDesktop' -or `$_.ExecutablePath -eq '$escapedCLI'
} | ForEach-Object { Stop-Process -Id `$_.ProcessId -Force -ErrorAction SilentlyContinue }
Remove-Item -LiteralPath '$escapedDesktop', '$escapedCLI' -Force
Remove-Item -LiteralPath '$escapedDesktop.install.json', '$escapedDesktop.install.lock' -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath '$escapedFolder\Solomon.lnk' -Force -ErrorAction SilentlyContinue
if ((Test-Path -LiteralPath '$escapedFolder') -and -not (Get-ChildItem -LiteralPath '$escapedFolder')) { Remove-Item -LiteralPath '$escapedFolder' }
Remove-Item -LiteralPath 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\Solomon' -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath `$PSCommandPath -Force
"@
[System.IO.File]::WriteAllText($uninstallPath, $uninstall, (New-Object System.Text.UTF8Encoding -ArgumentList $true))
$key = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\Solomon'
New-Item -Path $key -Force | Out-Null
$properties = @{
    DisplayName = 'Solomon'
    DisplayVersion = $Version
    Publisher = 'SAPPHIR3-ROS3'
    InstallLocation = $binDir
    DisplayIcon = "$DesktopPath,0"
    UninstallString = ('powershell.exe -NoProfile -ExecutionPolicy Bypass -File "' + $uninstallPath + '"')
}
foreach ($entry in $properties.GetEnumerator()) {
    New-ItemProperty -Path $key -Name $entry.Key -Value $entry.Value -PropertyType String -Force | Out-Null
}
New-ItemProperty -Path $key -Name NoModify -Value 1 -PropertyType DWord -Force | Out-Null
New-ItemProperty -Path $key -Name NoRepair -Value 1 -PropertyType DWord -Force | Out-Null
Write-Host 'Solomon registered in Windows apps and Start menu.'
