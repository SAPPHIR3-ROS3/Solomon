package test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestCreateFilesystemFolder(t *testing.T) {
	parent := t.TempDir()
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	endpoint := server.URL + "/__solomon/filesystem-directories"
	for index, name := range []string{"New folder", "New folder", "../escape", "..", "", "nested/folder", "nested\\folder"} {
		response := postJSONForServerTest(t, endpoint, map[string]string{"path": parent, "name": name})
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		expected := http.StatusBadRequest
		if index == 0 {
			expected = http.StatusCreated
		}
		if response.StatusCode != expected {
			t.Fatalf("%q: status %d, body %s", name, response.StatusCode, body)
		}
	}
	info, err := os.Stat(filepath.Join(parent, "New folder"))
	if err != nil || !info.IsDir() {
		t.Fatalf("folder not created: %v", err)
	}
}
