package test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestProjectFileImport(t *testing.T) {
	root := t.TempDir()
	endpoint := projectFileOperationEndpoint(t, root)
	external := filepath.Join(t.TempDir(), "external.bin")
	content := []byte{0, 255, 128, 13, 10, 42}
	if err := os.WriteFile(external, content, 0644); err != nil {
		t.Fatal(err)
	}
	operate := func(source, destination string, data *string, status int) {
		t.Helper()
		payload := map[string]any{"action": "import", "path": source, "destination": destination}
		if data != nil {
			payload["content"] = *data
		}
		body, _ := json.Marshal(payload)
		response, err := http.Post(endpoint, "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		result, _ := io.ReadAll(response.Body)
		if response.StatusCode != status {
			t.Fatalf("status %d, expected %d: %s", response.StatusCode, status, result)
		}
	}
	encoded := base64.StdEncoding.EncodeToString(content)
	operate(external, "external.bin", nil, 200)
	operate("upload.bin", "upload.bin", &encoded, 200)
	for _, name := range []string{external, filepath.Join(root, "external.bin"), filepath.Join(root, "upload.bin")} {
		actual, err := os.ReadFile(name)
		if err != nil || !bytes.Equal(actual, content) {
			t.Fatalf("content mismatch at %s: %v", name, err)
		}
	}
	operate(external, "external.bin", nil, 400)
	empty := ""
	operate("upload.bin", "upload.bin", &empty, 400)
	for _, destination := range []string{"../escape.bin", ".", filepath.Join(t.TempDir(), "escape.bin")} {
		operate(external, destination, nil, 400)
	}
	invalid := "!"
	operate("invalid", "invalid.bin", &invalid, 400)
	if err := os.Symlink(filepath.Dir(external), filepath.Join(root, "escape")); err == nil {
		operate("upload.bin", "escape/escaped.bin", &encoded, 400)
	}
	large := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{42}, 8192))
	operate("large.bin", "large.bin", &large, 200)
}
