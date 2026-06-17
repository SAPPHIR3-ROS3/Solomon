package cursor

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type BootstrapIO interface {
	Print(msg string)
}

func InstallDirReady(dir string) bool {
	if dir == "" {
		return false
	}
	if _, err := os.Stat(EntryScript(dir)); err != nil {
		return false
	}
	return true
}

func InstallRuntime(out BootstrapIO, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := extractBundle(dir); err != nil {
		return fmt.Errorf("extract cursor integration: %w", err)
	}
	if sdkInstalled(dir) {
		return nil
	}
	out.Print("installing Cursor integration with Cursor SDK")
	if err := npmEnsureProdDeps(dir); err != nil {
		return err
	}
	return nil
}

func InstallRuntimeClean(out BootstrapIO, dir string) error {
	if err := removeInstallDirRobust(dir); err != nil {
		return err
	}
	return InstallRuntime(out, dir)
}

func Bootstrap(out BootstrapIO, dir string) error {
	ready := InstallDirReady(dir)
	if ready {
		if _, err := os.Stat(filepath.Join(NodeModulesDir(dir), "@cursor", "sdk")); err == nil {
			return nil
		}
	}
	wasInstalled := ready
	if !wasInstalled {
		out.Print("Cursor integration not installed")
	}
	out.Print("installing Cursor integration with Cursor SDK")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := extractBundle(dir); err != nil {
		return fmt.Errorf("extract cursor integration: %w", err)
	}
	if err := npmEnsureProdDeps(dir); err != nil {
		return err
	}
	if !wasInstalled {
		out.Print("Cursor Integration ready")
	}
	return nil
}

func sdkInstalled(dir string) bool {
	_, err := os.Stat(filepath.Join(NodeModulesDir(dir), "@cursor", "sdk"))
	return err == nil
}

func extractBundle(dir string) error {
	if err := fs.WalkDir(bundleFS, "bundle", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("bundle", path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := bundleFS.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	}); err != nil {
		return err
	}
	npmrc := filepath.Join(dir, ".npmrc")
	if _, err := os.Stat(npmrc); os.IsNotExist(err) {
		if err := os.WriteFile(npmrc, []byte("loglevel=error\n"), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func npmEnsureProdDeps(dir string) error {
	if sdkInstalled(dir) {
		return nil
	}
	npm, err := exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("npm not found in PATH; install Node.js LTS (see scripts/install.sh): %w", err)
	}
	cmd := exec.Command(npm, "install", "--omit=dev")
	cmd.Dir = dir
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install failed in %s: %w", dir, err)
	}
	return nil
}

func removeInstallDirRobust(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	var last error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 300 * time.Millisecond)
		}
		last = os.RemoveAll(dir)
		if last == nil {
			return nil
		}
	}
	stale := dir + ".replacing-" + fmt.Sprint(time.Now().UnixNano())
	if err := os.Rename(dir, stale); err != nil {
		return fmt.Errorf("%w (could not remove or rename %s)", last, dir)
	}
	go func() { _ = os.RemoveAll(stale) }()
	return nil
}

func DevIntegrationDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir := wd
	for i := 0; i < 12; i++ {
		candidate := filepath.Join(dir, "integrations", "cursor")
		if InstallDirReady(candidate) {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func ResolveInstallDir() (string, error) {
	if d := strings.TrimSpace(os.Getenv("SOLOMON_CURSOR_API_ROOT")); d != "" {
		return d, nil
	}
	if d := DevIntegrationDir(); d != "" {
		return d, nil
	}
	return InstallDir()
}
