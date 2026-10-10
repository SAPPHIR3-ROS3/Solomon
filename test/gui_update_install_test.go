package test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func TestPreparedGUIUpdateRechecksIntegrityWithoutDownloadingBinary(t *testing.T) {
	t.Setenv("GOBIN", t.TempDir())
	const tag = "v2026.1009.0"
	const binary = "verified release binary"
	asset, err := updater.ReleaseAssetName(tag)
	if err != nil {
		t.Fatal(err)
	}
	downloads := 0
	restore := updater.SetHTTPDownload(func(_ context.Context, url string) (*http.Response, error) {
		body := binary
		if strings.HasSuffix(url, "/checksums.txt") {
			body = fmt.Sprintf("%x  %s\n", sha256.Sum256([]byte(binary)), asset)
		} else {
			downloads++
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	defer restore()
	staged, target, err := updater.PrepareInstall(context.Background(), tag, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(staged)
	if err := updater.VerifyPreparedInstall(context.Background(), tag, staged, target); err != nil {
		t.Fatal(err)
	}
	if downloads != 1 {
		t.Fatalf("binary downloaded %d times", downloads)
	}
	if err := updater.VerifyPreparedInstall(context.Background(), tag, staged, filepath.Join(t.TempDir(), "wrong-target")); err == nil {
		t.Fatal("accepted unexpected install destination")
	}
	if err := os.WriteFile(staged, []byte("tampered binary"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := updater.VerifyPreparedInstall(context.Background(), tag, staged, target); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("tampered download accepted: %v", err)
	}
}
