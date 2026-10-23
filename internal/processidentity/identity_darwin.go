package processidentity

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func Read(pid int) string {
	p, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil || int(p.Proc.P_pid) != pid || p.Proc.P_stat == 5 {
		return ""
	}
	return fmt.Sprintf("%d:%d", p.Proc.P_starttime.Sec, p.Proc.P_starttime.Usec)
}
