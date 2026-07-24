package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

type State struct {
	PID       int       `json:"pid"`
	URL       string    `json:"url"`
	LocalURL  string    `json:"localhost_url"`
	StartedAt time.Time `json:"started_at"`
	Version   string    `json:"version"`
	Mode      string    `json:"mode"`
	Vite      string    `json:"vite"`
	ViteURL   string    `json:"vite_url,omitempty"`
	DevDir    string    `json:"dev_directory,omitempty"`
	GOOS      string    `json:"goos"`
	GoVersion string    `json:"go_version"`
}

type Options struct {
	Mode       string
	DevDir     string
	ListenAddr string
}

const localPort = 8765

type Health struct {
	OK      bool      `json:"ok"`
	Server  State     `json:"server"`
	Now     time.Time `json:"now"`
	Uptime  string    `json:"uptime"`
	API     string    `json:"api"`
	GUI     string    `json:"gui"`
	Workers string    `json:"workers"`
}

func StatePath() (string, error) {
	home, err := paths.SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "run", "server", "state.json"), nil
}

func LogPath() (string, error) {
	home, err := paths.SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "logs", "server", "server.log"), nil
}

func LoadState() (State, error) {
	path, err := StatePath()
	if err != nil {
		return State{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("read server state: %w", err)
	}
	return state, nil
}

func SaveState(state State) error {
	path, err := StatePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func ClearState() error {
	path, err := StatePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func Run(ctx context.Context, options Options) error {
	listenAddr := options.ListenAddr
	if listenAddr == "" {
		listenAddr = "127.0.0.1:" + strconv.Itoa(localPort)
	}
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	mode := options.Mode
	if mode == "" {
		mode = "normal"
	}
	port := listener.Addr().(*net.TCPAddr).Port
	state := State{
		PID:       os.Getpid(),
		URL:       "http://127.0.0.1:" + strconv.Itoa(port),
		LocalURL:  "http://localhost:" + strconv.Itoa(port),
		StartedAt: time.Now().UTC(),
		Version:   commands.VersionString(),
		Mode:      mode,
		Vite:      "stopped",
		GOOS:      runtime.GOOS,
		GoVersion: runtime.Version(),
	}
	var vite *exec.Cmd
	var proxy *httputil.ReverseProxy
	if mode == "dev" {
		viteURL, command, err := startVite(options.DevDir)
		if err != nil {
			_ = listener.Close()
			return err
		}
		vite = command
		state.Vite = "running"
		state.ViteURL = viteURL.String()
		state.DevDir = options.DevDir
		proxy = httputil.NewSingleHostReverseProxy(viteURL)
	}
	defer func() {
		stopManagedProcess(vite)
	}()
	if err := SaveState(state); err != nil {
		_ = listener.Close()
		return err
	}
	fmt.Fprintf(os.Stderr, "solomon server started url=%s pid=%d mode=%s vite=%s\n", state.URL, state.PID, state.Mode, state.Vite)
	defer func() {
		fmt.Fprintln(os.Stderr, "solomon server stopped")
		_ = ClearState()
	}()

	httpServer := &http.Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, Health{
			OK: true, Server: state, Now: time.Now().UTC(), Uptime: time.Since(state.StartedAt).Round(time.Second).String(),
			API: "not configured", GUI: "not configured", Workers: "not configured",
		})
	})
	mux.HandleFunc("POST /_solomon/stop", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusAccepted, map[string]bool{"stopping": true})
		go func() { _ = httpServer.Shutdown(context.Background()) }()
	})
	if proxy != nil {
		mux.Handle("/", proxy)
	}
	httpServer.Handler = mux

	go func() {
		<-ctx.Done()
		_ = httpServer.Shutdown(context.Background())
	}()
	return httpServer.Serve(listener)
}

func startVite(directory string) (*url.URL, *exec.Cmd, error) {
	portListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	port := portListener.Addr().(*net.TCPAddr).Port
	_ = portListener.Close()
	viteURL, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(port))
	cmd := exec.Command("npm", "run", "dev", "--", "--host", "127.0.0.1", "--port", strconv.Itoa(port))
	cmd.Dir = directory
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	configureManagedProcess(cmd)
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	deadline := time.Now().Add(10 * time.Second)
	client := &http.Client{Timeout: 250 * time.Millisecond}
	for time.Now().Before(deadline) {
		response, err := client.Get(viteURL.String())
		if err == nil {
			_ = response.Body.Close()
			return viteURL, cmd, nil
		}
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	stopManagedProcess(cmd)
	return nil, nil, fmt.Errorf("Vite did not become ready; inspect with: solomon server logs")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
