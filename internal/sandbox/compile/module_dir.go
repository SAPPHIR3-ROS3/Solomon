package compile

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
)

const SolomonModulePath = "github.com/SAPPHIR3-ROS3/Solomon/v2026"

func ModuleDir() (string, error) {
	if p := strings.TrimSpace(os.Getenv("SOLOMON_MODULE_ROOT")); p != "" {
		root, err := validateModuleRoot(p)
		if err != nil {
			return "", fmt.Errorf("resolve module dir: SOLOMON_MODULE_ROOT: %w", err)
		}
		return root, nil
	}
	callerRoot, callerErr := moduleRootFromCaller()
	if callerErr == nil {
		return callerRoot, nil
	}
	stubRoot, stubErr := moduleRootFromStub()
	if stubErr == nil {
		return stubRoot, nil
	}
	return "", fmt.Errorf("resolve module dir: %s not found (caller: %v; stub: %v)", SolomonModulePath, callerErr, stubErr)
}

func validateModuleRoot(dir string) (string, error) {
	dir = filepath.Clean(dir)
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}
	if !goModDeclaresModule(string(b), SolomonModulePath) {
		return "", fmt.Errorf("%s is not module %s", dir, SolomonModulePath)
	}
	return dir, nil
}

func moduleRootFromCaller() (string, error) {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(1, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if frame.File != "" {
			if root := findModuleRootInTree(frame.File); root != "" {
				if validated, err := validateModuleRoot(root); err == nil {
					return validated, nil
				}
			}
		}
		if !more {
			break
		}
	}
	return "", errors.New("module root not found from install path")
}

func findModuleRootInTree(sourceFile string) string {
	dir := filepath.Dir(sourceFile)
	for {
		if root := moduleRootIfGoMod(dir); root != "" {
			return root
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func moduleRootIfGoMod(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}
	if goModDeclaresModule(string(b), SolomonModulePath) {
		return dir
	}
	return ""
}

func goModDeclaresModule(content, want string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "module ") {
			continue
		}
		mod := strings.TrimSpace(strings.TrimPrefix(line, "module "))
		mod = strings.Trim(mod, "`\"")
		return mod == want
	}
	return false
}

func moduleRootFromStub() (string, error) {
	version, err := solomonModuleVersion()
	if err != nil {
		return "", err
	}
	stubDir, err := ensureOrchestrateStub(version)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", SolomonModulePath)
	cmd.Dir = stubDir
	out, err := cmd.Output()
	if err != nil {
		if len(out) > 0 {
			err = fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
		}
		return "", err
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", errors.New("go list returned empty module dir")
	}
	return validateModuleRoot(root)
}

func solomonModuleVersion() (string, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", errors.New("no build info")
	}
	v := strings.TrimSpace(info.Main.Version)
	if v == "" || v == "(devel)" {
		return "", errors.New("devel build without local module tree")
	}
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	return v, nil
}

func ensureOrchestrateStub(version string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".solomon", "cache", "orchestrate-stub")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	goVer := "1.25.0"
	if info, ok := debug.ReadBuildInfo(); ok && strings.TrimSpace(info.GoVersion) != "" {
		goVer = strings.TrimPrefix(strings.TrimSpace(info.GoVersion), "go")
	}
	want := fmt.Sprintf("module solomon.orchestrate.stub\n\ngo %s\n\nrequire %s %s\n", goVer, SolomonModulePath, version)
	goMod := filepath.Join(dir, "go.mod")
	refresh := true
	if b, err := os.ReadFile(goMod); err == nil && string(b) == want {
		refresh = false
	}
	if refresh {
		if err := os.WriteFile(goMod, []byte(want), 0o600); err != nil {
			return "", err
		}
	}
	cmd := exec.Command("go", "mod", "download", SolomonModulePath+"@"+version)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return "", fmt.Errorf("orchestrate stub go mod download: %w: %s", err, msg)
		}
		return "", fmt.Errorf("orchestrate stub go mod download: %w", err)
	}
	return dir, nil
}
