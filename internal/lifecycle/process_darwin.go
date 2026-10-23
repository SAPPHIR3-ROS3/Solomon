package lifecycle

import (
	"bytes"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/processidentity"
	"golang.org/x/sys/unix"
)

func processIdentity(pid int) string { return processidentity.Read(pid) }

func processExecutable(pid int) string {
	data, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil || len(data) < 5 {
		return ""
	}
	end := bytes.IndexByte(data[4:], 0)
	if end < 0 {
		return ""
	}
	return string(data[4 : 4+end])
}
func installedDesktopProcesses(executable string) ([]desktopUpdateProcess, error) {
	processes, err := unix.SysctlKinfoProcSlice("kern.proc.uid", os.Getuid())
	if err != nil {
		return nil, err
	}
	var result []desktopUpdateProcess
	for _, p := range processes {
		pid := int(p.Proc.P_pid)
		if pid <= 1 || processExecutable(pid) != executable {
			continue
		}
		identity := processIdentity(pid)
		if identity != "" {
			result = append(result, desktopUpdateProcess{pid: pid, identity: identity, executable: executable, env: os.Environ()})
		}
	}
	return result, nil
}
