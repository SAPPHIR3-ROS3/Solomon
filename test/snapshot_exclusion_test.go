package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/filechanges"
)

func TestSnapshotExcludesOwnStorageAndAliases(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TMPDIR", root)
	if err := os.WriteFile(filepath.Join(root, "normal.txt"), []byte("normal"), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := filechanges.Capture(root)
	if err != nil {
		t.Fatal(err)
	}
	defer before.Close()
	dirs, _ := filepath.Glob(filepath.Join(root, "solomon-changes-*"))
	if len(dirs) != 1 {
		t.Fatalf("snapshot dirs: %v", dirs)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(dirs[0], alias); err != nil {
		t.Skip(err)
	}
	for _, targets := range [][]string{nil, {dirs[0]}, {alias}, {root}} {
		snapshot, err := filechanges.CapturePaths(root, targets)
		if err != nil {
			t.Fatal(err)
		}
		changes, err := filechanges.Compare(before, snapshot)
		snapshot.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, change := range changes {
			if filechanges.IsInternalSnapshotPath(change.Name) || change.Name == "alias" {
				t.Fatalf("tracked snapshot: %s", change.Name)
			}
		}
	}
}

func TestSessionLoadDropsOnlyInternalSnapshotChanges(t *testing.T) {
	internal := "/tmp/solomon-changes-123/abc"
	message := chatstore.Message{Role: "assistant", FileChanges: []chatstore.FileChangeSummary{{Path: internal, AddedLines: []string{"private copy"}}, {Path: "real.go", Added: 1}}}
	session := &chatstore.Session{FileChanges: map[string]chatstore.FileChangeLedger{internal: {}, "real.go": {}}, Messages: []chatstore.Message{message}}
	if !chatstore.FinishSessionLoad(session) {
		t.Fatal("cleanup not reported")
	}
	if len(session.FileChanges) != 1 || len(session.Messages[0].FileChanges) != 1 || session.Messages[0].FileChanges[0].Path != "real.go" {
		t.Fatalf("cleanup: %+v", session)
	}
}
