package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint/staging"
)

func TestStagingRestoreAfterPatchAndGoto(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "staging")
	store, err := staging.Load(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	proj := filepath.Join(dir, "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(proj, "f.txt")
	if err := os.WriteFile(file, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordBefore(file); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("after"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordOp(1, "patch", file, "", []byte("after")); err != nil {
		t.Fatal(err)
	}
	res, err := store.RestoreToCheckpoint(0, proj)
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesWritten != 1 {
		t.Fatalf("written: %d", res.FilesWritten)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "before" {
		t.Fatalf("got %q", b)
	}
}

func TestStagingRestoreRename(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "staging")
	store, err := staging.Load(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	proj := filepath.Join(dir, "proj")
	src := filepath.Join(proj, "old.txt")
	dst := filepath.Join(proj, "new.txt")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordBefore(src); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(src, dst); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordOp(1, "rename", src, dst, []byte("data")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RestoreToCheckpoint(0, proj); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("dst should be gone: %v", err)
	}
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "data" {
		t.Fatalf("got %q", b)
	}
}
