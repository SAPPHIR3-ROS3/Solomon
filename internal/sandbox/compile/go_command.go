package compile

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Desktop launchers do not inherit the PATH configured by an interactive shell.
// Keep the user's PATH preference, then try Go's installation roots directly.
func goCommand(args ...string) (*exec.Cmd, error) {
	if path, err := exec.LookPath("go"); err == nil {
		return orchestrateGoCommand(path, args...), nil
	}
	name := "go"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	roots := []string{os.Getenv("GOROOT"), runtime.GOROOT()}
	switch runtime.GOOS {
	case "windows":
		if dir := os.Getenv("ProgramFiles"); dir != "" {
			roots = append(roots, filepath.Join(dir, "Go"))
		}
	case "darwin":
		roots = append(roots, "/usr/local/go", "/opt/homebrew/opt/go/libexec", "/usr/local/opt/go/libexec")
	default:
		roots = append(roots, "/usr/local/go", "/usr/lib/go")
	}
	for _, root := range roots {
		if root == "" || !filepath.IsAbs(root) {
			continue
		}
		path := filepath.Join(root, "bin", name)
		if resolved, err := exec.LookPath(path); err == nil {
			return orchestrateGoCommand(resolved, args...), nil
		}
	}
	return nil, fmt.Errorf("orchestrate: Go toolchain not found in PATH or installation roots; install Go or set GOROOT to its installation directory")
}

func orchestrateGoCommand(path string, args ...string) *exec.Cmd {
	cmd := exec.Command(path, args...)
	cmd.Env = orchestrateGoEnv()
	return cmd
}

// Select the exact toolchain from Solomon's embedded go.mod for Code Mode only.
// Go downloads that release when it is not already installed. In particular, an old
// Windows installation or a globally pinned GOTOOLCHAIN must not make the
// generated module compile with an older Go than Solomon requires.
func orchestrateGoEnv() []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "GOTOOLCHAIN") || strings.EqualFold(key, "GOWORK") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "GOTOOLCHAIN=go"+RequiredGoVersion(), "GOWORK=off")
}
