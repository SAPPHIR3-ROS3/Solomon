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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

type State struct {
	PID       int                `json:"pid"`
	URL       string             `json:"url"`
	LocalURL  string             `json:"localhost_url"`
	Addresses []ReachableAddress `json:"addresses,omitempty"`
	StartedAt time.Time          `json:"started_at"`
	Version   string             `json:"version"`
	Mode      string             `json:"mode"`
	Vite      string             `json:"vite"`
	ViteURL   string             `json:"vite_url,omitempty"`
	DevDir    string             `json:"dev_directory,omitempty"`
	GOOS      string             `json:"goos"`
	GoVersion string             `json:"go_version"`
}

type ReachableAddress struct {
	Kind      string `json:"kind"`
	Interface string `json:"interface,omitempty"`
	URL       string `json:"url"`
}

type Options struct {
	Mode       string
	DevDir     string
	ListenAddr string
}

const serverPortEnv = "SOLOMON_SERVER_PORT"

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
	_ = os.Remove(path + ".tmp")
	var removeErr error
	for attempt := 0; attempt < 20; attempt++ {
		removeErr = os.Remove(path)
		if removeErr == nil || errors.Is(removeErr, os.ErrNotExist) {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return removeErr
}

func Run(ctx context.Context, options Options) error {
	mode := options.Mode
	if mode == "" {
		mode = "normal"
	}
	listenAddr, err := resolveListenAddr(options)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp4", listenAddr)
	if err != nil {
		return err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	addresses := reachableAddresses(listener.Addr().(*net.TCPAddr), port)
	state := State{
		PID:       os.Getpid(),
		URL:       "http://127.0.0.1:" + strconv.Itoa(port),
		LocalURL:  "http://localhost:" + strconv.Itoa(port),
		Addresses: addresses,
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

	serviceCtx, cancelService := context.WithCancel(ctx)
	defer cancelService()
	terminalManager := newTerminalManager(serviceCtx.Done())
	defer terminalManager.CloseAll()
	terminalAPI := newTerminalAPI(terminalManager)
	httpServer := &http.Server{}
	mux := http.NewServeMux()
	chatAPI := newChatAPI(serviceCtx)
	projectAPI := newProjectAPI()
	customizationAPI := newCustomizationAPI()
	modelAPI := newModelAPI()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, Health{
			OK: true, Server: state, Now: time.Now().UTC(), Uptime: time.Since(state.StartedAt).Round(time.Second).String(),
			API: "not configured", GUI: "not configured", Workers: "not configured",
		})
	})
	mux.HandleFunc("POST /_solomon/stop", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusAccepted, map[string]bool{"stopping": true})
		cancelService()
		go func() { _ = httpServer.Shutdown(context.Background()) }()
	})
	mux.HandleFunc(chatAPIPath, chatAPI.handleProjects)
	mux.HandleFunc(chatAPIPath+"/", func(w http.ResponseWriter, r *http.Request) {
		if chatAPI.handlesProjectRoute(r.URL.Path) {
			chatAPI.handleProjectRoute(w, r)
			return
		}
		if projectAPI.handlesProjectRoute(r.URL.Path) {
			projectAPI.handleProjectRoute(w, r)
			return
		}
		if proxy != nil {
			proxy.ServeHTTP(w, r)
			return
		}
		writeAPIError(w, http.StatusNotFound, os.ErrNotExist)
	})
	mux.HandleFunc("/__solomon/fast-mode", chatAPI.handleFastMode)
	mux.HandleFunc("/__solomon/home-directories", projectAPI.handleHomeDirectoryEntries)
	mux.HandleFunc("/__solomon/home-git-branches", projectAPI.handleHomeBranches)
	mux.HandleFunc("/__solomon/home-git-worktrees", projectAPI.handleHomeWorktrees)
	mux.HandleFunc("/__solomon/home-git-checkout", projectAPI.handleHomeCheckout)
	mux.HandleFunc("/__solomon/user-name", projectAPI.handleUserName)
	mux.HandleFunc("/__solomon/reasoning-effort", projectAPI.handleReasoningEffort)
	mux.HandleFunc("/__solomon/rules", customizationAPI.handleRules)
	mux.HandleFunc("/__solomon/rules/reorder", customizationAPI.handleReorderRules)
	mux.HandleFunc("/__solomon/rules/update", customizationAPI.handleUpdateRule)
	mux.HandleFunc("/__solomon/rules/delete", customizationAPI.handleDeleteRule)
	mux.HandleFunc("/__solomon/skills", customizationAPI.handleSkills)
	mux.HandleFunc("/__solomon/mcps", customizationAPI.handleMCPs)
	mux.HandleFunc("/__solomon/subagents", customizationAPI.handleSubagents)
	mux.HandleFunc("/__solomon/subagents/update", customizationAPI.handleUpdateSubagent)
	mux.HandleFunc("/__solomon/subagents/delete", customizationAPI.handleDeleteSubagent)
	mux.HandleFunc("/__solomon/roles-table", customizationAPI.handleRolesTable)
	mux.HandleFunc("/__solomon/promptTemplates", customizationAPI.handlePromptTemplates)
	mux.HandleFunc("/__solomon/promptTemplate", customizationAPI.handlePromptTemplate)
	mux.HandleFunc("/__solomon/promptTemplates/update", customizationAPI.handleUpdatePromptTemplate)
	mux.HandleFunc("/__solomon/promptTemplates/reset", customizationAPI.handleResetPromptTemplate)
	mux.HandleFunc("/__solomon/models", modelAPI.handleCatalog)
	mux.HandleFunc("/__solomon/current-model", modelAPI.handleCurrent)
	mux.HandleFunc("/__solomon/model-visibility", modelAPI.handleVisibility)
	mux.HandleFunc("/__solomon/connect-provider", modelAPI.handleConnectProvider)
	mux.HandleFunc("/__solomon/terminal", terminalAPI.handleWebSocket)
	if proxy != nil {
		mux.Handle("/", proxy)
	}
	httpServer.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__solomon" || strings.HasPrefix(r.URL.Path, "/__solomon/") {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin != "" {
				if !terminalOriginAllowed(r) {
					if r.Method == http.MethodOptions {
						writeAPIError(w, http.StatusForbidden, errors.New("request origin is not allowed"))
						return
					}
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
				}
			}
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			if r.Method == http.MethodOptions {
				if origin == "" {
					writeAPIError(w, http.StatusForbidden, errors.New("request origin is required"))
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		mux.ServeHTTP(w, r)
	})

	go func() {
		<-serviceCtx.Done()
		_ = httpServer.Shutdown(context.Background())
	}()
	return httpServer.Serve(listener)
}

func resolveListenAddr(options Options) (string, error) {
	if options.ListenAddr != "" {
		return options.ListenAddr, nil
	}

	port := strings.TrimSpace(os.Getenv(serverPortEnv))
	if port == "" {
		return "0.0.0.0:0", nil
	}
	parsedPort, err := strconv.Atoi(port)
	if err != nil || parsedPort < 1 || parsedPort > 65535 {
		return "", fmt.Errorf("%s must be a TCP port between 1 and 65535", serverPortEnv)
	}
	return net.JoinHostPort("0.0.0.0", strconv.Itoa(parsedPort)), nil
}

func reachableAddresses(listenerAddr *net.TCPAddr, port int) []ReachableAddress {
	if listenerAddr == nil {
		return nil
	}

	addresses := make([]ReachableAddress, 0)
	seen := make(map[string]bool)
	appendAddress := func(ip net.IP, interfaceName string) {
		ip = ip.To4()
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
			return
		}
		key := ip.String()
		if seen[key] {
			return
		}
		seen[key] = true
		kind := "local"
		if isTailscaleIPv4(ip) {
			kind = "tailscale"
		}
		addresses = append(addresses, ReachableAddress{
			Kind:      kind,
			Interface: interfaceName,
			URL:       "http://" + net.JoinHostPort(ip.String(), strconv.Itoa(port)),
		})
	}

	listenerIP := listenerAddr.IP.To4()
	if listenerIP == nil || listenerIP.IsLoopback() {
		return nil
	}
	if !listenerIP.IsUnspecified() {
		appendAddress(listenerIP, interfaceNameForIP(listenerIP))
		return addresses
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, networkInterface := range interfaces {
		interfaceAddrs, err := networkInterface.Addrs()
		if err != nil {
			continue
		}
		for _, interfaceAddr := range interfaceAddrs {
			appendAddress(interfaceIPv4(interfaceAddr), networkInterface.Name)
		}
	}

	sort.Slice(addresses, func(i, j int) bool {
		if addresses[i].Kind != addresses[j].Kind {
			return addresses[i].Kind == "local"
		}
		if addresses[i].Interface != addresses[j].Interface {
			return addresses[i].Interface < addresses[j].Interface
		}
		return addresses[i].URL < addresses[j].URL
	})
	return addresses
}

func interfaceIPv4(address net.Addr) net.IP {
	switch value := address.(type) {
	case *net.IPNet:
		return value.IP.To4()
	case *net.IPAddr:
		return value.IP.To4()
	}

	text := address.String()
	if host, _, err := net.SplitHostPort(text); err == nil {
		return net.ParseIP(host).To4()
	}
	return net.ParseIP(text).To4()
}

func interfaceNameForIP(target net.IP) string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, networkInterface := range interfaces {
		interfaceAddrs, err := networkInterface.Addrs()
		if err != nil {
			continue
		}
		for _, interfaceAddr := range interfaceAddrs {
			if ip := interfaceIPv4(interfaceAddr); ip != nil && ip.Equal(target) {
				return networkInterface.Name
			}
		}
	}
	return ""
}

func isTailscaleIPv4(ip net.IP) bool {
	ip = ip.To4()
	return ip != nil && ip[0] == 100 && ip[1] >= 64 && ip[1] <= 127
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
