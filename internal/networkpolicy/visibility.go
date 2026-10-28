package networkpolicy

import "strings"

// Filter the user-facing list only; hiding an interface must not alter routing
// or server access through Docker bridges.
func visibleInterfaces(rows []Interface, inContainer bool) []Interface {
	if inContainer {
		return rows
	}
	result := make([]Interface, 0, len(rows))
	for _, row := range rows {
		if !containerInterface(row.Name) {
			result = append(result, row)
		}
	}
	return result
}

func containerInterface(name string) bool {
	for _, prefix := range []string{"docker", "veth", "cni", "flannel", "podman"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	// Docker Compose names its bridges br-<12 hexadecimal digits>.
	suffix, ok := strings.CutPrefix(name, "br-")
	if !ok || len(suffix) != 12 {
		return false
	}
	for _, char := range suffix {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f') {
			return false
		}
	}
	return true
}

func runningInContainer(readFile func(string) ([]byte, error)) bool {
	for _, path := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := readFile(path); err == nil {
			return true
		}
	}
	for _, path := range []string{"/proc/self/cgroup", "/proc/1/cgroup"} {
		data, err := readFile(path)
		if err != nil {
			continue
		}
		for _, marker := range []string{"/docker/", "/docker-", "docker-", "/kubepods/", "kubepods-", "libpod-", "cri-containerd-"} {
			if strings.Contains(string(data), marker) {
				return true
			}
		}
	}
	return false
}
