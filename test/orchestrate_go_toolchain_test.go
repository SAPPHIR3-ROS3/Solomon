package test

import (
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
)

func TestOrchestrateSelectsProjectToolchain(t *testing.T) {
	// Neither a globally pinned old release nor an unrelated workspace may
	// override the toolchain selected from Solomon's embedded go.mod.
	t.Setenv("GOTOOLCHAIN", "go1.27.0")
	t.Setenv("GOWORK", filepath.Join(t.TempDir(), "missing.work"))
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	wasm, err := compile.BuildWASM(compile.Options{
		Source:     "package main\nfunc main() {}\n",
		ModuleRoot: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(wasm) < 4 || string(wasm[:4]) != "\x00asm" {
		t.Fatal("expected a compiled WASM module")
	}
}
