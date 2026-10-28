package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkhttp"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	docscorpus "github.com/SAPPHIR3-ROS3/Solomon/v2026/docs"
	guibundle "github.com/SAPPHIR3-ROS3/Solomon/v2026/gui"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logo"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/processidentity"
)

// Frontend transitions never restart the daemon that owns the active chat.
// Requests keep their current handler while the replacement is prepared.
type developmentFrontend struct {
	ctx        context.Context
	transition sync.Mutex
	mu         sync.RWMutex
	state      State
	handler    http.Handler
	process    *exec.Cmd
}

func (d *developmentFrontend) snapshot() State {
	d.mu.RLock()
	defer d.mu.RUnlock()
	state := d.state
	if d.process != nil && supervisedViteExited(d.process) {
		state.Vite = "stopped"
	}
	return state
}

func (d *developmentFrontend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.mu.RLock()
	handler := d.handler
	d.mu.RUnlock()
	if handler == nil {
		http.NotFound(w, r)
		return
	}
	handler.ServeHTTP(w, r)
}

func (d *developmentFrontend) close() {
	d.transition.Lock()
	defer d.transition.Unlock()
	stopManagedProcess(d.process)
}

func exportGUISources() (string, error) {
	home, err := paths.SolomonHome()
	if err != nil {
		return "", err
	}
	return exportGUISourcesTo(filepath.Join(home, "customization", "gui"))
}

// PrepareGUIWorkspace reuses GUI sources or initializes an empty chosen folder.
func PrepareGUIWorkspace(raw string) (string, error) {
	if directory, err := config.ValidateDevDirectory(raw); err == nil {
		return directory, nil
	}
	directory, err := config.NormalizeDevDirectory(raw)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(directory)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if len(entries) != 0 {
		return "", fmt.Errorf("choose an empty folder or an existing Solomon GUI source directory; files in %s were preserved", directory)
	}
	// The chosen folder contains the complete base, including shared inputs.
	// No files are created outside the selected folder.
	return exportGUISourcesTo(filepath.Join(directory, "gui"))
}

func exportGUISourcesTo(directory string) (string, error) {
	// Never overwrite an existing customization, including a partial one.
	if _, err := os.Stat(directory); err == nil {
		return config.ValidateDevDirectory(directory)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(directory), 0700); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(filepath.Dir(directory), ".gui-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	err = fs.WalkDir(guibundle.Sources(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		target := filepath.Join(stage, filepath.FromSlash(path))
		if entry.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		data, err := fs.ReadFile(guibundle.Sources(), path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0600)
	})
	if err != nil {
		return "", err
	}
	icon, err := fs.ReadFile(guibundle.Assets(), "icon.svg")
	if err != nil {
		return "", fmt.Errorf("read bundled GUI icon: %w", err)
	}
	if err := os.WriteFile(filepath.Join(stage, "icon.svg"), icon, 0600); err != nil {
		return "", err
	}
	// The frontend imports the shared welcome logo and documentation using
	// repository-relative paths. Export those inputs alongside gui as well.
	shared := map[string][]byte{"internal/logo/logo.txt": []byte(logo.ASCII), "internal/logo/colors.txt": []byte(logo.ColorsRaw)}
	err = fs.WalkDir(docscorpus.Corpus, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		data, err := fs.ReadFile(docscorpus.Corpus, path)
		if err != nil {
			return err
		}
		shared["docs/"+path] = data
		return nil
	})
	if err != nil {
		return "", err
	}
	for path, data := range shared {
		target := filepath.Join(filepath.Dir(directory), filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return "", err
		}
		file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, writeErr := file.Write(data)
		closeErr := file.Close()
		if writeErr != nil {
			return "", writeErr
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	if err := os.Rename(stage, directory); err != nil {
		return "", err
	}
	return directory, nil
}

// PrepareGUIDependencies installs the frontend toolchain before activation.
func PrepareGUIDependencies(ctx context.Context, directory string) error {
	node, err := exec.LookPath("node")
	if err != nil {
		return fmt.Errorf("GUI customization requires Node.js with npm: %w", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "node_modules", "vite", "bin", "vite.js")); err == nil {
		return nil
	}
	// npm's JS entry avoids cmd.exe quoting and visible Windows consoles.
	npm, err := exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("GUI customization requires npm: %w", err)
	}
	args := []string{"install", "--include=dev", "--no-audit", "--no-fund"}
	if _, err := os.Stat(filepath.Join(directory, "package-lock.json")); err == nil {
		args[0] = "ci"
	}
	cmd := exec.CommandContext(ctx, npm, args...)
	for _, base := range []string{filepath.Dir(node), filepath.Dir(npm)} {
		entry := filepath.Join(base, "node_modules", "npm", "bin", "npm-cli.js")
		if _, err := os.Stat(entry); err == nil {
			cmd = exec.CommandContext(ctx, node, append([]string{entry}, args...)...)
			break
		}
	}
	cmd.Dir = directory
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	configureManagedProcess(cmd)
	fmt.Fprintln(os.Stderr, "preparing GUI customization dependencies in", directory)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("GUI dependency installation failed (current interface preserved): %w; inspect solomon server logs", err)
	}
	return nil
}

func (d *developmentFrontend) change(ctx context.Context, mode, directory string) (State, error) {
	d.transition.Lock()
	defer d.transition.Unlock()
	previous := d.snapshot()
	if err := ctx.Err(); err != nil {
		return previous, err
	}
	if mode != "dev" && mode != "normal" {
		return previous, fmt.Errorf("mode must be dev or normal")
	}
	if mode == "dev" {
		var err error
		if directory == "" {
			directory = previous.DevDir
			if directory == "" {
				if cfg, loadErr := config.Load(); loadErr == nil {
					directory = cfg.Server.DevDirectory
				}
			}
			if directory == "" {
				directory, err = exportGUISources()
			}
		}
		if err != nil {
			return previous, err
		}
		directory, err = PrepareGUIWorkspace(directory)
		if err != nil {
			return previous, err
		}
		if previous.Mode == "dev" && previous.DevDir == directory {
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, previous.ViteURL, nil)
			if err == nil {
				response, probeErr := (&http.Client{Transport: networkhttp.NewDefaultTransport(), Timeout: time.Second}).Do(request)
				if probeErr == nil {
					response.Body.Close()
					if response.StatusCode >= 200 && response.StatusCode < 300 {
						return previous, nil
					}
				}
			}
		}
		if err := PrepareGUIDependencies(ctx, directory); err != nil {
			return previous, err
		}
	}
	next := previous
	next.ViteIdentity = ""
	next.Mode, next.DevDir, next.Vite, next.ViteURL, next.VitePID = mode, "", "stopped", "", 0
	var process *exec.Cmd
	var handler http.Handler = productionFrontend()
	if mode == "dev" {
		address, command, err := startVite(directory)
		if err != nil {
			return previous, err
		}
		process, handler = command, httputil.NewSingleHostReverseProxy(address)
		next.DevDir, next.Vite, next.ViteURL, next.VitePID = directory, "running", address.String(), managedVitePID(command)
		next.ViteIdentity = processidentity.Read(next.VitePID)
	}
	if err := ctx.Err(); err != nil {
		stopManagedProcess(process)
		return previous, err
	}
	// Persist only after Vite is ready; startup failures leave preferences intact.
	patch := map[string]json.RawMessage{"server.dev_mode": json.RawMessage(fmt.Sprint(mode == "dev"))}
	if mode == "dev" {
		patch["server.dev_directory"], _ = json.Marshal(directory)
	}
	if err := SaveState(next); err != nil {
		stopManagedProcess(process)
		return previous, err
	}
	if _, err := config.UpdateGUIPreferences(config.GUISettingsPatch{Advanced: patch}); err != nil {
		_ = SaveState(previous)
		stopManagedProcess(process)
		return previous, err
	}
	d.mu.Lock()
	old := d.process
	d.state, d.handler, d.process = next, handler, process
	d.mu.Unlock()
	stopManagedProcess(old)
	return next, nil
}

func (d *developmentFrontend) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, d.snapshot())
		return
	}
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	if !terminalOriginAllowed(r) {
		writeAPIError(w, http.StatusForbidden, fmt.Errorf("request origin is not allowed"))
		return
	}
	var request struct {
		Mode      string `json:"mode"`
		Directory string `json:"directory"`
	}
	if err := decodeJSONBody(w, r, &request, 16<<10); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	if d.ctx != nil {
		stop := context.AfterFunc(d.ctx, cancel)
		defer stop()
	}
	if request.Mode == "prepare" {
		d.transition.Lock()
		directory, err := PrepareGUIWorkspace(request.Directory)
		d.transition.Unlock()
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"directory": directory})
		return
	}
	state, err := d.change(ctx, request.Mode, request.Directory)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}
