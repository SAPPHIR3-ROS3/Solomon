package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

const checksumsAsset = "checksums.txt"

func releaseChecksumsURL(tag string) string {
	return releaseDownloadURL(tag, checksumsAsset)
}

func expectedChecksum(checksumsBody, assetName string) (string, bool) {
	for _, line := range strings.Split(checksumsBody, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hash := strings.ToLower(fields[0])
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		if name == assetName {
			return hash, true
		}
	}
	return "", false
}

func fileSHA256Hex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func verifyReleaseAsset(ctx context.Context, tag, asset, filePath string, progress io.Writer) error {
	if progress == nil {
		progress = io.Discard
	}
	url := releaseChecksumsURL(tag)
	resp, err := httpDownload(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		logging.Log(logging.WARNING_LOG_LEVEL, "updater checksums missing; skipping verify", logging.LogOptions{Params: map[string]any{"tag": tag}})
		fmt.Fprintf(progress, "Warning: release %s has no %s; skipping integrity check\n", tag, checksumsAsset)
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	expected, ok := expectedChecksum(string(body), asset)
	if !ok {
		return fmt.Errorf("checksums: no entry for %s in %s", asset, checksumsAsset)
	}
	actual, err := fileSHA256Hex(filePath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(expected, actual) {
		err := fmt.Errorf("checksum mismatch for %s (expected %s, got %s)", asset, expected, actual)
		logging.Log(logging.ERROR_LOG_LEVEL, "updater checksum mismatch", logging.LogOptions{Params: map[string]any{"tag": tag, "asset": asset, "expected": expected, "actual": actual}})
		return err
	}
	return nil
}
