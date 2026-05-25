package cursor

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// InstallRuntime copies the embedded bundle into dir and ensures npm prod deps for the sidecar.
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

// InstallRuntimeClean removes any previous runtime under dir, redeploys the embedded bundle, and reinstalls prod npm deps.
func InstallRuntimeClean(out BootstrapIO, dir string) error {
	if err := os.RemoveAll(dir); err != nil {
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
	return fs.WalkDir(bundleFS, "bundle", func(path string, d fs.DirEntry, err error) error {
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
	})
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
