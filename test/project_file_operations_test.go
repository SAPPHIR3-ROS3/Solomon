package test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

type projectFileOperationRequest struct {
	Action      string `json:"action"`
	Path        string `json:"path"`
	Destination string `json:"destination,omitempty"`
}

func projectFileOperationEndpoint(t *testing.T, root string) string {
	t.Helper()
	server, stop := startServerForTest(t, serverruntime.Options{})
	t.Cleanup(stop)
	projectID := strings.Repeat("a", 64)
	mapPath, err := paths.ProjectsMapPath()
	if err != nil {
		t.Fatal(err)
	}
	// Preserve the supplied path, including platform-specific temporary-directory
	// aliases, so the HTTP API must correctly resolve the workspace boundary.
	payload, err := json.Marshal(map[string]string{root: projectID})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mapPath, payload, 0600); err != nil {
		t.Fatal(err)
	}
	return server.URL + "/__solomon/projects/" + projectID + "/file-operation"
}

func runProjectFileOperation(t *testing.T, endpoint string, operation projectFileOperationRequest, wantStatus int) {
	t.Helper()
	response := postJSONForServerTest(t, endpoint, operation)
	defer response.Body.Close()
	if response.StatusCode != wantStatus {
		var payload map[string]any
		// Read the API error so failures explain the path validation or filesystem error.
		decodeServerTestJSON(t, response, &payload)
		t.Fatalf("%+v: status = %d, want %d: %v", operation, response.StatusCode, wantStatus, payload)
	}
	// Drain successful test responses so the HTTP transport can reuse the
	// connection instead of leaving new connections pending during shutdown.
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		t.Fatal(err)
	}
}

func TestProjectFileOperations(t *testing.T) {
	root := t.TempDir()
	endpoint := projectFileOperationEndpoint(t, root)
	if err := os.WriteFile(filepath.Join(root, "original.txt"), []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []projectFileOperationRequest{
		{Action: "copy", Path: "original.txt", Destination: "copy.txt"},
		{Action: "rename", Path: "copy.txt", Destination: "renamed.txt"},
		{Action: "move", Path: "renamed.txt", Destination: "moved.txt"},
	} {
		runProjectFileOperation(t, endpoint, operation, http.StatusOK)
	}
	content, err := os.ReadFile(filepath.Join(root, "moved.txt"))
	if err != nil || string(content) != "content" {
		t.Fatalf("content: %q, %v", content, err)
	}
	for _, action := range []string{"copy", "rename", "move"} {
		runProjectFileOperation(t, endpoint, projectFileOperationRequest{Action: action, Path: "original.txt", Destination: "moved.txt"}, http.StatusBadRequest)
	}
	runProjectFileOperation(t, endpoint, projectFileOperationRequest{Action: "delete", Path: "moved.txt"}, http.StatusOK)
	if _, err := os.Stat(filepath.Join(root, "moved.txt")); !os.IsNotExist(err) {
		t.Fatalf("delete failed: %v", err)
	}
	if content, err := os.ReadFile(filepath.Join(root, "original.txt")); err != nil || string(content) != "content" {
		t.Fatalf("original changed: %q, %v", content, err)
	}
}

func TestProjectFileOperationsRejectUnsafePaths(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	endpoint := projectFileOperationEndpoint(t, root)
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "outside.txt"), []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	operations := []projectFileOperationRequest{
		{Action: "delete", Path: "."},
		{Action: "rename", Path: "file.txt", Destination: "../escape.txt"},
		{Action: "copy", Path: "file.txt", Destination: filepath.Join(outside, "escape.txt")},
		{Action: "move", Path: "file.txt", Destination: "."},
		{Action: "unknown", Path: "file.txt"},
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err == nil {
		operations = append(operations, projectFileOperationRequest{Action: "delete", Path: "escape/outside.txt"}, projectFileOperationRequest{Action: "copy", Path: "file.txt", Destination: "escape/copy.txt"})
	} else {
		t.Logf("symbolic link cases unavailable: %v", err)
	}
	for _, operation := range operations {
		runProjectFileOperation(t, endpoint, operation, http.StatusBadRequest)
	}
	content, err := os.ReadFile(filepath.Join(outside, "outside.txt"))
	if err != nil || string(content) != "outside" {
		t.Fatalf("modified outside file: %q, %v", content, err)
	}
}

func TestProjectFolderCopy(t *testing.T) {
	root := t.TempDir()
	endpoint := projectFileOperationEndpoint(t, root)
	if err := os.MkdirAll(filepath.Join(root, "source", "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "source", "nested", "file"), []byte("nested"), 0600); err != nil {
		t.Fatal(err)
	}
	runProjectFileOperation(t, endpoint, projectFileOperationRequest{Action: "copy", Path: "source", Destination: "source/nested/copy"}, http.StatusBadRequest)
	runProjectFileOperation(t, endpoint, projectFileOperationRequest{Action: "copy", Path: "source", Destination: "copy"}, http.StatusOK)
	if content, err := os.ReadFile(filepath.Join(root, "copy", "nested", "file")); err != nil || string(content) != "nested" {
		t.Fatalf("copy failed: %q, %v", content, err)
	}
}

func TestProjectFileOperationsResolveWorkspaceAliases(t *testing.T) {
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "workspace-alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	endpoint := projectFileOperationEndpoint(t, alias)
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	response, err := http.Get(strings.TrimSuffix(endpoint, "/file-operation") + "/file?path=file.txt")
	if err != nil {
		t.Fatal(err)
	}
	content, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || readErr != nil || string(content) != "content" {
		t.Fatalf("read through alias: status=%d content=%q error=%v", response.StatusCode, content, readErr)
	}
	runProjectFileOperation(t, endpoint, projectFileOperationRequest{Action: "copy", Path: "file.txt", Destination: "copy.txt"}, http.StatusOK)
	runProjectFileOperation(t, endpoint, projectFileOperationRequest{Action: "delete", Path: "."}, http.StatusBadRequest)
	if content, err := os.ReadFile(filepath.Join(root, "copy.txt")); err != nil || string(content) != "content" {
		t.Fatalf("copy through alias failed: %q, %v", content, err)
	}
}
