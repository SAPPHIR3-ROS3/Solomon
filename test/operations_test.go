package test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func operationCall(t *testing.T, env *tools.Env, name string, args map[string]any) (any, error) {
	t.Helper()
	args["intent"] = "verify workspace operation"
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	return tools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: name, Args: raw})
}

func TestHTTPRequestOperations(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/target", http.StatusFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if r.Method != http.MethodPatch || r.Header.Get("X-Test") != "yes" || !bytes.Equal(body, []byte{0, 255, 1}) {
			t.Errorf("unexpected request: %s %v %v", r.Method, r.Header, body)
		}
		w.Header().Set("X-Result", "kept")
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte{0, 255, 2, 3})
	}))
	defer server.Close()
	env := &tools.Env{AllowDeferredTools: true}
	result, err := operationCall(t, env, "httpRequest", map[string]any{"url": server.URL, "method": "PATCH", "headers": map[string]string{"X-Test": "yes"}, "bodyBase64": base64.StdEncoding.EncodeToString([]byte{0, 255, 1}), "maxBytes": 3})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	body, err := base64.StdEncoding.DecodeString(m["bodyBase64"].(string))
	if err != nil || !bytes.Equal(body, []byte{0, 255, 2}) || m["status"] != 422 || m["truncated"] != true {
		t.Fatalf("unexpected response: %#v", m)
	}
	result, err = operationCall(t, env, "httpRequest", map[string]any{"url": server.URL + "/redirect"})
	if err != nil || result.(map[string]any)["status"] != 302 || requests.Load() != 2 {
		t.Fatalf("redirect followed: %v %v requests=%d", result, err, requests.Load())
	}
	for _, args := range []map[string]any{{"url": "file:///tmp/file"}, {"url": server.URL, "maxBytes": -1}, {"url": server.URL, "timeoutSeconds": 121}, {"url": server.URL, "body": "x", "bodyBase64": "eA=="}, {"url": server.URL, "bodyBase64": "!"}} {
		if _, err := operationCall(t, env, "httpRequest", args); err == nil {
			t.Fatalf("accepted invalid options: %v", args)
		}
	}
}

func TestFileMetadataAndCopyOperations(t *testing.T) {
	root := t.TempDir()
	env := &tools.Env{ProjRoot: root, AllowDeferredTools: true}
	var checkpointPath, checkpointKind string
	env.CheckpointRecordEdit = func(kind, path, destination string, content []byte) {
		checkpointPath, checkpointKind = path, kind
	}
	data := []byte{0, 255, 3, 0}
	if err := os.WriteFile(filepath.Join(root, "source"), data, 0o750); err != nil {
		t.Fatal(err)
	}
	result, err := operationCall(t, env, "stat", map[string]any{"path": "missing"})
	if err != nil || result.(map[string]any)["exists"] != false {
		t.Fatalf("missing stat: %v %v", result, err)
	}
	if _, err := operationCall(t, env, "copyFile", map[string]any{"path": "source", "destination": "nested/copy"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "nested/copy"))
	if checkpointPath != filepath.Join(root, "nested/copy") || checkpointKind != "create" {
		t.Fatalf("copy checkpoint missing: %q %q", checkpointPath, checkpointKind)
	}
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("copy corrupted: %v %v", got, err)
	}
	if _, err := operationCall(t, env, "copyFile", map[string]any{"path": "source", "destination": "nested/copy"}); err == nil {
		t.Fatal("copy overwrote destination")
	}
	if _, err := operationCall(t, env, "copyFile", map[string]any{"path": "nested", "destination": "directory-copy"}); err == nil {
		t.Fatal("directory copy accepted")
	}
	result, err = operationCall(t, env, "stat", map[string]any{"path": "nested/copy"})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	if m["exists"] != true || m["type"] != "file" || m["size"] != int64(4) || m["modified"] == "" {
		t.Fatalf("unexpected metadata: %v", m)
	}
	if err := os.Symlink("absent", filepath.Join(root, "link")); err == nil {
		result, err = operationCall(t, env, "stat", map[string]any{"path": "link"})
		if err != nil || result.(map[string]any)["type"] != "symlink" {
			t.Fatalf("symlink stat: %v %v", result, err)
		}
	}
}

func TestCreateDirOperations(t *testing.T) {
	root := t.TempDir()
	env := &tools.Env{ProjRoot: root, AllowDeferredTools: true}
	for range 2 {
		if _, err := operationCall(t, env, "createDir", map[string]any{"path": "empty/nested"}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "empty/nested"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("empty directory: %v %v", entries, err)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"", "file", "file/nested"} {
		if _, err := operationCall(t, env, "createDir", map[string]any{"path": path}); err == nil {
			t.Fatalf("accepted invalid directory path %q", path)
		}
	}
	content, err := os.ReadFile(filepath.Join(root, "file"))
	if err != nil || string(content) != "unchanged" {
		t.Fatalf("existing file changed: %q %v", content, err)
	}
}

func TestOperationsDiscoveryAndGuards(t *testing.T) {
	for _, name := range []string{"httpRequest", "stat", "copyFile", "createDir"} {
		_, err := operationCall(t, &tools.Env{}, name, map[string]any{})
		if err == nil || !strings.Contains(err.Error(), "not available") {
			t.Fatalf("direct call allowed: %s %v", name, err)
		}
		_, err = tools.Exec(context.Background(), &tools.Env{AllowDeferredTools: true}, "agent", tooling.Invocation{Name: name, Args: json.RawMessage(`{}`)})
		if err == nil || !strings.Contains(err.Error(), "intent") {
			t.Fatalf("missing intent accepted: %s %v", name, err)
		}
		result, err := operationCall(t, &tools.Env{}, "searchTools", map[string]any{"query": name})
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(result)
		if !strings.Contains(string(raw), name) {
			t.Fatalf("tool not discoverable: %s %s", name, raw)
		}
	}
}
