package test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "solomon-tui-tests-*")
	if err != nil {
		panic(err)
	}
	os.Setenv("SOLOMON_TUI_CONFIG", filepath.Join(root, "interface.json"))
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}
