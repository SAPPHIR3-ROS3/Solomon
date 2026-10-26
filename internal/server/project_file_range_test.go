package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectFileByteRanges(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	root := t.TempDir()
	id := strings.Repeat("a", 64)
	mapping, _ := json.Marshal(map[string]string{root: id})
	if err := os.WriteFile(filepath.Join(home, "projectsId.json"), mapping, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "large.pdf"), []byte("%PDF-1.4\n0123456789"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		rangeHeader string
		status      int
		body        string
	}{
		{"bytes=0-7", 206, "%PDF-1.4"},
		{"bytes=-4", 206, "6789"},
		{"bytes=100-200", 416, ""},
	} {
		t.Run(tc.rangeHeader, func(t *testing.T) {
			request := httptest.NewRequest("GET", "/?path=large.pdf", nil)
			request.Header.Set("Range", tc.rangeHeader)
			response := httptest.NewRecorder()
			(&projectAPI{}).handleProjectFile(response, request, id)
			if response.Code != tc.status {
				t.Fatalf("status %d: %s", response.Code, response.Body.String())
			}
			if tc.status == 206 && response.Body.String() != tc.body {
				t.Fatalf("body %q", response.Body.String())
			}
			if tc.status == 206 && response.Header().Get("Accept-Ranges") != "bytes" {
				t.Fatal("missing range support")
			}
		})
	}
}

func TestProjectFileRejectsPDFTextSave(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	root := t.TempDir()
	id := strings.Repeat("a", 64)
	mapping, _ := json.Marshal(map[string]string{root: id})
	if err := os.WriteFile(filepath.Join(home, "projectsId.json"), mapping, 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "document.PDF")
	original := "%PDF-1.4 binary data"
	if err := os.WriteFile(target, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("PUT", "/?path=document.PDF", strings.NewReader("corrupted"))
	response := httptest.NewRecorder()
	(&projectAPI{}).handleProjectFile(response, request, id)
	if response.Code != 415 {
		t.Fatalf("status %d", response.Code)
	}
	content, err := os.ReadFile(target)
	if err != nil || string(content) != original {
		t.Fatalf("PDF changed: %q, %v", content, err)
	}
}
