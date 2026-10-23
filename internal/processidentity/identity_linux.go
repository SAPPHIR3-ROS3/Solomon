package processidentity

import (
	"fmt"
	"os"
	"strings"
)

func Read(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return ""
	}
	end := strings.LastIndex(string(data), ")")
	if end < 0 {
		return ""
	}
	fields := strings.Fields(string(data)[end+1:])
	if len(fields) < 20 || fields[0] == "Z" {
		return ""
	}
	return fields[19] // starttime distinguishes a reused PID.
}
