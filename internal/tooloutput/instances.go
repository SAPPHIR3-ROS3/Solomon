package tooloutput

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

func instancesDir() (string, error) {
	root, err := paths.SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "instances"), nil
}

func deferredTempFile() (string, error) {
	root, err := paths.SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "temp.txt"), nil
}

func RegisterInstance(pid int) error {
	if pid <= 0 {
		return nil
	}
	dir, err := instancesDir()
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "tool output instances dir resolve failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "tool output instances mkdir failed", logging.LogOptions{Params: map[string]any{"dir": dir, "err": err.Error()}})
		return err
	}
	_ = pruneStaleInstances(dir)
	path := filepath.Join(dir, fmt.Sprintf("%d.instance", pid))
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", pid)), 0o600)
}

func UnregisterInstance(pid int) error {
	if pid <= 0 {
		return nil
	}
	dir, err := instancesDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, fmt.Sprintf("%d.instance", pid))
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func ActiveOtherInstances(currentPID int) int {
	dir, err := instancesDir()
	if err != nil {
		return 0
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".instance")
		pid, err := strconv.Atoi(name)
		if err != nil {
			_ = os.Remove(filepath.Join(dir, e.Name()))
			continue
		}
		if pid == currentPID {
			continue
		}
		if processAlive(pid) {
			n++
		} else {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	return n
}

func pruneStaleInstances(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".instance")
		pid, err := strconv.Atoi(name)
		if err != nil || !processAlive(pid) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	return nil
}

func appendDeferredTempCleanup(projectHex string) error {
	projectHex = strings.TrimSpace(projectHex)
	if projectHex == "" {
		return nil
	}
	path, err := deferredTempFile()
	if err != nil {
		return err
	}
	existing, _ := readDeferredTempCleanups()
	for _, h := range existing {
		if h == projectHex {
			return nil
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", projectHex)
	return err
}

func readDeferredTempCleanups() ([]string, error) {
	path, err := deferredTempFile()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	seen := map[string]struct{}{}
	for _, line := range strings.Split(string(b), "\n") {
		h := strings.TrimSpace(line)
		if h == "" {
			continue
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}
	return out, nil
}

func clearDeferredTempFile() error {
	path, err := deferredTempFile()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func flushDeferredTempCleanups(currentHex string, currentSpilled bool) error {
	hexes, err := readDeferredTempCleanups()
	if err != nil {
		return err
	}
	if currentSpilled {
		hexes = appendUniqueHex(hexes, currentHex)
	}
	for _, h := range hexes {
		if err := CleanupProjectTemp(h); err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "deferred tool temp cleanup failed", logging.LogOptions{Params: map[string]any{"project": h, "err": err.Error()}})
		}
	}
	return clearDeferredTempFile()
}

func appendUniqueHex(list []string, hex string) []string {
	hex = strings.TrimSpace(hex)
	if hex == "" {
		return list
	}
	for _, h := range list {
		if h == hex {
			return list
		}
	}
	return append(list, hex)
}

func CloseProjectTemp(projectHex string, othersActive, spillGenerated bool) error {
	if othersActive {
		if spillGenerated {
			return appendDeferredTempCleanup(projectHex)
		}
		return nil
	}
	return flushDeferredTempCleanups(projectHex, spillGenerated)
}
