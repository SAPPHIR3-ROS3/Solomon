package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
)

func TestModuleDir_fromNonModuleCWD(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	root, err := compile.ModuleDir()
	if err != nil {
		t.Fatal(err)
	}
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("missing go.mod in %q: %v", root, err)
	}
	if !strings.Contains(string(mod), compile.SolomonModulePath) {
		t.Fatalf("go.mod in %q is not Solomon module: %s", root, mod)
	}
}

func TestBuildWASM_fromNonModuleCWD(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	src := `package main

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/sdk"

func main() {
	_, _ = sdk.Glob("**/*", "list project files")
}
`
	if _, err := compile.BuildWASM(compile.Options{Source: src}); err != nil {
		t.Fatal(err)
	}
}

func TestBuildWASM_readOnlyModule(t *testing.T) {
	for _, inModuleCache := range []bool{false, true} {
		name := "source-tree"
		if inModuleCache {
			name = "module-cache"
		}
		t.Run(name, func(t *testing.T) {
			cacheRoot := t.TempDir()
			if inModuleCache {
				t.Setenv("GOMODCACHE", cacheRoot)
			}
			testBuildWASMReadOnlyModule(t, filepath.Join(cacheRoot, "Solomon module"))
		})
	}
}

func testBuildWASMReadOnlyModule(t *testing.T, root string) {
	t.Helper()
	sdkDir := filepath.Join(root, "internal", "sandbox", "sdk")
	if err := os.MkdirAll(sdkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":                      "module " + compile.SolomonModulePath + "\n\ngo 1.25.0\n",
		"internal/sandbox/sdk/sdk.go": "package sdk\nconst Value = 42\n",
		// Also prevent writes on platforms where directory modes are ignored.
		".solomon": "module contents must not be modified\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o444); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })
	wasm, err := compile.BuildWASM(compile.Options{
		ModuleRoot: root,
		Source: `package main
import ("fmt"; "sdk")
func main() { fmt.Println(sdk.Value) }
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(wasm) < 4 || string(wasm[:4]) != "\x00asm" {
		t.Fatal("expected a compiled WASM module")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("module directory was modified: %v", entries)
	}
}
