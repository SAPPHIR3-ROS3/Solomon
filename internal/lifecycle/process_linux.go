package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func processIdentity(pid int) string {
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

func installedDesktopProcesses(executable string) ([]desktopUpdateProcess, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	var processes []desktopUpdateProcess
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 1 {
			continue
		}
		root := filepath.Join("/proc", entry.Name())
		info, err := os.Stat(root)
		if err != nil || int(info.Sys().(*syscall.Stat_t).Uid) != os.Getuid() {
			continue
		}
		path, err := os.Readlink(filepath.Join(root, "exe"))
		if err != nil || strings.TrimSuffix(path, " (deleted)") != executable {
			continue
		}
		env, err := os.ReadFile(filepath.Join(root, "environ"))
		if err != nil {
			return nil, err
		}
		cwd, err := os.Readlink(filepath.Join(root, "cwd"))
		if err != nil {
			return nil, err
		}
		identity := processIdentity(pid)
		if identity != "" {
			processes = append(processes, desktopUpdateProcess{pid: pid, identity: identity, executable: executable, env: strings.Split(strings.TrimSuffix(string(env), "\x00"), "\x00"), cwd: cwd})
		}
	}
	return processes, nil
}

func processExecutable(pid int) string {
	p, _ := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	return strings.TrimSuffix(p, " (deleted)")
}
