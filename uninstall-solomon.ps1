$ErrorActionPreference = 'Stop'
& 'D:\Projects\Golang\Solomon\solomon-upgrade-fix.exe' server stop
Get-CimInstance Win32_Process | Where-Object {
    $_.ExecutablePath -eq 'D:\Projects\Golang\Solomon\solomon-desktop.exe' -or $_.ExecutablePath -eq 'D:\Projects\Golang\Solomon\solomon-upgrade-fix.exe'
} | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
Remove-Item -LiteralPath 'D:\Projects\Golang\Solomon\solomon-desktop.exe', 'D:\Projects\Golang\Solomon\solomon-upgrade-fix.exe' -Force
Remove-Item -LiteralPath 'D:\Projects\Golang\Solomon\solomon-desktop.exe.install.json', 'D:\Projects\Golang\Solomon\solomon-desktop.exe.install.lock' -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath 'C:\Users\patri\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\Solomon\Solomon.lnk' -Force -ErrorAction SilentlyContinue
if ((Test-Path -LiteralPath 'C:\Users\patri\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\Solomon') -and -not (Get-ChildItem -LiteralPath 'C:\Users\patri\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\Solomon')) { Remove-Item -LiteralPath 'C:\Users\patri\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\Solomon' }
Remove-Item -LiteralPath 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\Solomon' -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $PSCommandPath -Force