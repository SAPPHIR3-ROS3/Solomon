package test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectFileByteRanges(t *testing.T) {
	root := t.TempDir()
	endpoint := strings.TrimSuffix(projectFileOperationEndpoint(t, root), "/file-operation") + "/file?path=large.pdf"
	if err := os.WriteFile(filepath.Join(root, "large.pdf"), []byte("%PDF-1.4\n0123456789"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		rangeHeader string
		status      int
		body        string
	}{
		{"bytes=0-7", http.StatusPartialContent, "%PDF-1.4"},
		{"bytes=-4", http.StatusPartialContent, "6789"},
		{"bytes=100-200", http.StatusRequestedRangeNotSatisfiable, ""},
	} {
		t.Run(tc.rangeHeader, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodGet, endpoint, nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Range", tc.rangeHeader)
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != tc.status {
				t.Fatalf("status %d: %s", response.StatusCode, body)
			}
			if tc.status == http.StatusPartialContent {
				if string(body) != tc.body {
					t.Fatalf("body %q, want %q", body, tc.body)
				}
				if response.Header.Get("Accept-Ranges") != "bytes" {
					t.Fatal("missing range support")
				}
				if response.Header.Get("Content-Type") != "application/pdf" {
					t.Fatal("incorrect PDF content type")
				}
			}
		})
	}
}

func TestProjectFileRejectsPDFTextSave(t *testing.T) {
	root := t.TempDir()
	endpoint := strings.TrimSuffix(projectFileOperationEndpoint(t, root), "/file-operation") + "/file?path=document.PDF"
	target := filepath.Join(root, "document.PDF")
	original := "%PDF-1.4 binary data"
	if err := os.WriteFile(target, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPut, endpoint, strings.NewReader("corrupted"))
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("status %d", response.StatusCode)
	}
	content, err := os.ReadFile(target)
	if err != nil || string(content) != original {
		t.Fatalf("PDF changed: %q, %v", content, err)
	}
}
