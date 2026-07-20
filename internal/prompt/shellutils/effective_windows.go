//go:build windows

package shellutils

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func windowsEffective() string {
	if p := detectPowerShell7(); p != "" {
		return p
	}
	if p := shellFromParentChain(); p != "" {
		return p
	}
	if strings.TrimSpace(os.Getenv("PSModulePath")) != "" {
		if p := windowsFallbackExecutable(); p != "" {
			return p
		}
	}
	if v := strings.TrimSpace(os.Getenv("COMSPEC")); v != "" {
		return v
	}
	return "unknown"
}

func detectPowerShell7() string {
	if strings.TrimSpace(os.Getenv("PWSH_VERSION")) == "" {
		return ""
	}
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
	return ""
}

func shellFromParentChain() string {
	parents, err := processParentMap()
	if err != nil {
		return ""
	}
	pid := uint32(os.Getppid())
	for step := 0; step < 24 && pid > 0; step++ {
		exe, err := exePathForPID(int(pid))
		if err == nil && exe != "" {
			switch strings.ToLower(filepath.Base(exe)) {
			case "powershell.exe", "pwsh.exe", "cmd.exe":
				return filepath.Clean(exe)
			}
		}
		next := parents[pid]
		if next == 0 || next == pid {
			break
		}
		pid = next
	}
	return ""
}

func processParentMap() (map[uint32]uint32, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)
	parents := make(map[uint32]uint32)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snapshot, &pe); err != nil {
		return nil, err
	}
	for {
		parents[pe.ProcessID] = pe.ParentProcessID
		if err := windows.Process32Next(snapshot, &pe); err != nil {
			break
		}
	}
	return parents, nil
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
