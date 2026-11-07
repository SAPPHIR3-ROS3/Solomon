package memoryprofile

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/processidentity"
)

type Process struct {
	PID        int    `json:"pid"`
	Identity   string `json:"identity"`
	Executable string `json:"executable"`
	Kind       string `json:"kind"`
}

func init() { collectProcesses = linuxProcesses }

func linuxProcesses(home string) (Usage, Usage, Usage, bool) {
	server, ok := readUsage(os.Getpid(), processidentity.Read(os.Getpid()))
	var desktop, webkit Usage
	entries, _ := os.ReadDir(filepath.Join(home, "run", "clients"))
	seen := map[int]bool{os.Getpid(): true}
	for _, entry := range entries {
		if len(seen) >= 256 {
			break
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		file, err := os.Open(filepath.Join(home, "run", "clients", entry.Name()))
		if err != nil {
			continue
		}
		var root Process
		err = json.NewDecoder(io.LimitReader(file, 64*1024)).Decode(&root)
		file.Close()
		if err != nil || root.Kind != "desktop" || seen[root.PID] || !verifiedRoot(root) {
			continue
		}
		usage, valid := readUsage(root.PID, root.Identity)
		if !valid {
			continue
		}
		seen[root.PID] = true
		addUsage(&desktop, usage)
		walkWebKit(root.PID, root.Identity, seen, &webkit, 0)
	}
	return server, desktop, webkit, ok
}

func verifiedRoot(root Process) bool {
	if root.PID <= 1 || root.Identity == "" || processidentity.Read(root.PID) != root.Identity {
		return false
	}
	switch filepath.Base(root.Executable) {
	case "solomon", "solomon-desktop", "solomon-standalone":
	default:
		return false
	}
	actual, err := os.Stat(fmt.Sprintf("/proc/%d/exe", root.PID))
	expected, other := os.Stat(root.Executable)
	return err == nil && other == nil && os.SameFile(actual, expected)
}

func walkWebKit(pid int, identity string, seen map[int]bool, usage *Usage, depth int) {
	if depth > 16 || len(seen) > 256 || processidentity.Read(pid) != identity {
		return
	}
	tasks, _ := os.ReadDir(fmt.Sprintf("/proc/%d/task", pid))
	for _, task := range tasks {
		data, _ := os.ReadFile(fmt.Sprintf("/proc/%d/task/%s/children", pid, task.Name()))
		for _, value := range strings.Fields(string(data)) {
			child, err := strconv.Atoi(value)
			if len(seen) >= 256 {
				return
			}
			if err != nil || seen[child] {
				continue
			}
			childIdentity := processidentity.Read(child)
			if childIdentity == "" || parentPID(child) != pid || processidentity.Read(pid) != identity {
				continue
			}
			seen[child] = true
			executable, _ := os.Readlink(fmt.Sprintf("/proc/%d/exe", child))
			switch filepath.Base(executable) {
			case "WebKitWebProcess", "WebKitNetworkProcess", "WebKitGPUProcess":
				if next, valid := readUsage(child, childIdentity); valid && parentPID(child) == pid {
					addUsage(usage, next)
				}
			}
			switch filepath.Base(executable) {
			case "bwrap", "WebKitWebProcess", "WebKitNetworkProcess", "WebKitGPUProcess":
				walkWebKit(child, childIdentity, seen, usage, depth+1)
			}
		}
	}
}

func parentPID(pid int) int {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0
	}
	end := strings.LastIndex(string(data), ")")
	if end < 0 {
		return 0
	}
	fields := strings.Fields(string(data)[end+1:])
	if len(fields) < 2 {
		return 0
	}
	parent, _ := strconv.Atoi(fields[1])
	return parent
}

func readUsage(pid int, identity string) (Usage, bool) {
	if identity == "" {
		return Usage{}, false
	}
	root := fmt.Sprintf("/proc/%d", pid)
	info, err := os.Stat(root)
	if err != nil || int(info.Sys().(*syscall.Stat_t).Uid) != os.Getuid() {
		return Usage{}, false
	}
	data, err := os.ReadFile(root + "/status")
	if err != nil {
		return Usage{}, false
	}
	var usage Usage
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		value, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "VmRSS:":
			usage.RSS = value * 1024
		case "VmSwap:":
			usage.Swap = value * 1024
		}
	}
	if processidentity.Read(pid) != identity {
		return Usage{}, false
	}
	usage.Processes = 1
	return usage, true
}

func addUsage(total *Usage, next Usage) {
	total.Processes += next.Processes
	total.RSS += next.RSS
	total.Swap += next.Swap
}
