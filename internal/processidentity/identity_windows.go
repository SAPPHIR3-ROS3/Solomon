package processidentity

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func Read(pid int) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	var code uint32
	if windows.GetExitCodeProcess(h, &code) != nil || code != 259 {
		return ""
	}
	var creation, exit, kernel, user windows.Filetime
	if windows.GetProcessTimes(h, &creation, &exit, &kernel, &user) != nil {
		return ""
	}
	return fmt.Sprintf("%d:%d", creation.HighDateTime, creation.LowDateTime)
}
