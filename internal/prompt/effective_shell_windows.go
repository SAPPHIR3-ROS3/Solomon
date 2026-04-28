//go:build windows

package prompt

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func windowsInteractiveShellOverride() string {
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
	ppid := windows.Getppid()
	if ppid <= 0 {
		return ""
	}
	parentExe, err := exePathForPID(ppid)
	if err != nil || parentExe == "" {
		return ""
	}
	switch strings.ToLower(filepath.Base(parentExe)) {
	case "powershell.exe", "pwsh.exe":
		return filepath.Clean(parentExe)
	default:
		return ""
	}
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
