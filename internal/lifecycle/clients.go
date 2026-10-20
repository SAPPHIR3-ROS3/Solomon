package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

type clientRecord struct {
	PID        int      `json:"pid"`
	Identity   string   `json:"identity"`
	Executable string   `json:"executable"`
	Kind       string   `json:"kind"`
	Args       []string `json:"args"`
	Cwd        string   `json:"cwd"`
	Env        []string `json:"env"`
}

// RegisterClient provides a graceful stop handshake independent of OS signals.
// The coordinator verifies both the executable and process creation time.
func RegisterClient(ctx context.Context, kind string, args []string) (<-chan struct{}, func(), error) {
	home, err := paths.SolomonHome()
	if err != nil {
		return nil, nil, err
	}
	directory := filepath.Join(home, "run", "clients")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, nil, err
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	record := clientRecord{PID: os.Getpid(), Identity: processIdentity(os.Getpid()), Executable: executable, Kind: kind, Args: args, Cwd: cwd}
	if record.Identity == "" {
		return nil, nil, fmt.Errorf("cannot identify Solomon client process")
	}
	for _, name := range []string{"DISPLAY", "WAYLAND_DISPLAY", "XDG_RUNTIME_DIR", "DBUS_SESSION_BUS_ADDRESS", "SOLOMON_HOME", "SOLOMON_BINARY", "SOLOMON_SERVER_PORT"} {
		if value, ok := os.LookupEnv(name); ok {
			record.Env = append(record.Env, name+"="+value)
		}
	}
	file, err := os.CreateTemp(directory, "client-*.tmp")
	if err != nil {
		return nil, nil, err
	}
	name := file.Name()
	data, err := json.Marshal(record)
	if err == nil {
		_, err = file.Write(data)
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(name)
		return nil, nil, err
	}
	published := strings.TrimSuffix(name, ".tmp") + ".json"
	if err := os.Rename(name, published); err != nil {
		os.Remove(name)
		return nil, nil, err
	}
	name = published
	watchCtx, cancel := context.WithCancel(ctx)
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-watchCtx.Done():
				return
			case <-ticker.C:
				if _, err := os.Stat(name + ".stop"); err == nil {
					close(stop)
					return
				}
			}
		}
	}()
	cleanup := func() { cancel(); os.Remove(name); os.Remove(name + ".stop") }
	return stop, cleanup, nil
}

func updateClients(executable string) ([]desktopUpdateProcess, error) {
	home, err := paths.SolomonHome()
	if err != nil {
		return nil, err
	}
	directory := filepath.Join(home, "run", "clients")
	entries, err := os.ReadDir(directory)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var result []desktopUpdateProcess
	seen := map[int]bool{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := filepath.Join(directory, entry.Name())
		data, err := os.ReadFile(name)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		var r clientRecord
		if json.Unmarshal(data, &r) != nil || r.PID <= 1 || r.Identity == "" {
			continue
		}
		if processIdentity(r.PID) != r.Identity || !sameExecutable(processExecutable(r.PID), r.Executable) {
			os.Remove(name)
			os.Remove(name + ".stop")
			continue
		}
		base := strings.ToLower(filepath.Base(r.Executable))
		if r.Kind == "desktop" && base != desktopName() {
			continue
		}
		if r.Kind == "tui" && (runtime.GOOS != "windows" || base != "solomon.exe") {
			continue
		}
		if r.Kind != "desktop" && r.Kind != "tui" {
			continue
		}
		env := append(os.Environ(), r.Env...)
		result = append(result, desktopUpdateProcess{pid: r.PID, identity: r.Identity, executable: r.Executable, args: r.Args, cwd: r.Cwd, env: env, stopFile: name + ".stop", kind: r.Kind})
		seen[r.PID] = true
	}
	legacy, err := installedDesktopProcesses(executable)
	if err != nil {
		return nil, err
	}
	for _, p := range legacy {
		if !seen[p.pid] {
			result = append(result, p)
		}
	}
	return result, nil
}

func sameExecutable(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return a == b
}
func requestClientStop(client desktopUpdateProcess) error {
	if processIdentity(client.pid) != client.identity {
		return nil
	}
	if client.stopFile != "" {
		return os.WriteFile(client.stopFile, []byte("update\n"), 0600)
	}
	return stopLegacyDesktop(client.pid)
}
