package logging

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func init() { processMemory = linuxProcessMemory }

func linuxProcessMemory(params map[string]any) {
	for path, keys := range map[string]map[string]string{
		"/proc/vmstat":      {"oom_kill": "system_oom_kills"},
		"/proc/meminfo":     {"MemAvailable:": "system_available_kib", "SwapFree:": "system_swap_free_kib"},
		"/proc/self/status": {"VmRSS:": "parent_rss_kib", "VmSwap:": "parent_swap_kib"},
	} {
		readMemoryCounters(path, keys, params)
	}
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		if group, ok := strings.CutPrefix(line, "0::/"); ok {
			path := filepath.Join("/sys/fs/cgroup", filepath.Clean("/"+group), "memory.events")
			readMemoryCounters(path, map[string]string{"oom": "cgroup_oom_events", "oom_kill": "cgroup_oom_kills", "max": "cgroup_memory_limit_events"}, params)
		}
	}
}

func readMemoryCounters(path string, keys map[string]string, params map[string]any) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if key, ok := keys[fields[0]]; ok {
			if value, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
				params[key] = value
			}
		}
	}
}
