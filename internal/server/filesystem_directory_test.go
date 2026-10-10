package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateFilesystemFolder(t *testing.T) {
	parent := t.TempDir()
	api := &projectAPI{}
	for index, name := range []string{"New folder", "New folder", "../escape", "..", "", "nested/folder", "nested\\folder"} {
		body, _ := json.Marshal(map[string]string{"path": parent, "name": name})
		response := httptest.NewRecorder()
		api.handleFilesystemDirectoryEntries(response, httptest.NewRequest("POST", "/__solomon/filesystem-directories", bytes.NewReader(body)))
		expected := 400
		if index == 0 {
			expected = 201
		}
		if response.Code != expected {
			t.Fatalf("%q: status %d, body %s", name, response.Code, response.Body.String())
		}
	}
	info, err := os.Stat(filepath.Join(parent, "New folder"))
	if err != nil || !info.IsDir() {
		t.Fatalf("folder not created: %v", err)
	}
}
