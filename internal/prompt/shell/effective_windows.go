//go:build windows

package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func windowsEffective() string {
	if strings.TrimSpace(os.Getenv("PWSH_VERSION")) != "" {
		if p, err := exec.LookPath("pwsh.exe"); err == nil && p != "" {
			return filepath.Clean(p)
		}
		for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
			base = strings.TrimSpace(base)
			if base == "" {
				continue
			}
			p := filepath.Join(base, "PowerShell", "7", "pwsh.exe")
			if isExecutableFile(p) {
				return filepath.Clean(p)
			}
		}
	}
	if v := strings.TrimSpace(os.Getenv("COMSPEC")); v != "" {
		return v
	}
	ppid := windows.Getppid()
	if ppid <= 0 {
		if p := windowsFallbackExecutable(); p != "" {
			return p
		}
		return "unknown"
	}
	parentExe, err := exePathForPID(ppid)
	if err != nil || parentExe == "" {
		return ""
	}
	switch strings.ToLower(filepath.Base(parentExe)) {
	case "powershell.exe", "pwsh.exe":
		return filepath.Clean(parentExe)
	default:
		if p := windowsFallbackExecutable(); p != "" {
			return p
		}
		return "unknown"
	}
}

func windowsFallbackExecutable() string {
	systemRoot := strings.TrimSpace(os.Getenv("SystemRoot"))
	if systemRoot == "" {
		systemRoot = strings.TrimSpace(os.Getenv("windir"))
	}
	if systemRoot != "" {
		for _, rel := range []string{`System32\WindowsPowerShell\v1.0\powershell.exe`, `SysWOW64\WindowsPowerShell\v1.0\powershell.exe`} {
			p := filepath.Join(systemRoot, rel)
			if isExecutableFile(p) {
				return p
			}
		}
	}
	for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		base = strings.TrimSpace(base)
		if base == "" {
			continue
		}
		p := filepath.Join(base, "PowerShell", "7", "pwsh.exe")
		if isExecutableFile(p) {
			return filepath.Clean(p)
		}
	}
	return ""
}

func exePathForPID(pid int) (string, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_PATH)
	n := uint32(len(buf))
	err = windows.QueryFullProcessImageName(h, 0, &buf[0], &n)
	if err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf[:n]), nil
}

func isExecutableFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}
