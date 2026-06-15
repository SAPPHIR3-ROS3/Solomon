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
	_, _ = sdk.Glob("**/*")
}
`
	if _, err := compile.BuildWASM(compile.Options{Source: src}); err != nil {
		t.Fatal(err)
	}
}
