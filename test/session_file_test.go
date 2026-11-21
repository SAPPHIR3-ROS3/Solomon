package test

import (
	"encoding/json"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestWriteActiveRegistryFileReplacesCompleteSnapshot(t *testing.T) {
	path := sessionSnapshotPath(t)
	for _, content := range []string{`{"messages":[]}`, `{"messages":["new"]}`} {
		if err := writeActiveRegistrySnapshot(path, []byte(content)); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil || snapshotContent(t, got) != content {
			t.Fatalf("snapshot = %q, err = %v", got, err)
		}
	}
	files, _ := filepath.Glob(path + ".*.tmp")
	if len(files) != 0 {
		t.Fatalf("staging files left after success: %v", files)
	}
}

func TestWriteActiveRegistryFileConcurrentWriters(t *testing.T) {
	path := sessionSnapshotPath(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := writeActiveRegistrySnapshot(path, []byte(strings.Repeat(string(rune('a'+i)), 65536))); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshotContent(t, data)) != 65536 || strings.Trim(snapshotContent(t, data), snapshotContent(t, data)[:1]) != "" {
		t.Fatal("concurrent writes published an incomplete or mixed snapshot")
	}
}

func TestWriteActiveRegistryFileRetainsSnapshotOnPublishFailure(t *testing.T) {
	path := sessionSnapshotPath(t)
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	// A nonempty directory cannot be replaced by a file on any supported OS.
	if err := os.WriteFile(filepath.Join(path, "keep"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	err := writeActiveRegistrySnapshot(path, []byte("unsaved"))
	if err == nil {
		t.Fatal("expected publication failure")
	}
	files, _ := filepath.Glob(path + ".*.tmp")
	if len(files) != 1 || !strings.Contains(err.Error(), files[0]) {
		t.Fatalf("snapshot missing from error: %v, files %v", err, files)
	}
	data, readErr := os.ReadFile(files[0])
	if readErr != nil || snapshotContent(t, data) != "unsaved" {
		t.Fatalf("snapshot = %q, err = %v", data, readErr)
	}
	data, readErr = os.ReadFile(filepath.Join(path, "keep"))
	if readErr != nil || string(data) != "original" {
		t.Fatal("original destination changed")
	}
}

// Exercise persistence through its production API, including retained staging files.
func sessionSnapshotPath(t *testing.T) string {
	t.Helper()
	setSolomonHome(t, t.TempDir())
	path, err := paths.ActiveSubagentsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}
func writeActiveRegistrySnapshot(_ string, content []byte) error {
	return chatstore.WriteActiveSubagents(&chatstore.ActiveSubagentsFile{Agents: []chatstore.ActiveSubagentEntry{{ID: "snapshot", Status: string(content)}}})
}
func snapshotContent(t *testing.T, data []byte) string {
	t.Helper()
	var snapshot chatstore.ActiveSubagentsFile
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	return snapshot.Agents[0].Status
}
