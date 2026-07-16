//go:build automatic_role_scores

package benchmarks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

const (
	refreshInterval   = 7 * 24 * time.Hour
	scoresAssetName   = "scores.json"
	manifestAssetName = "manifest.json"
)

type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type releaseAssetsJSON struct {
	TagName string         `json:"tag_name"`
	Assets  []ReleaseAsset `json:"assets"`
}

var httpGetRelease = func(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "solomon-benchmarks")
	return http.DefaultClient.Do(req)
}

var httpDownloadAsset = func(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "solomon-benchmarks")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
}

func NeedsRefresh(st *Store) bool {
	if st == nil {
		return true
	}
	if len(st.Scores.Models) < 16 {
		return true
	}
	if strings.TrimSpace(st.Manifest.Commit) == "seed" {
		return true
	}
	t, ok := ManifestTime(st.Manifest)
	if !ok {
		return true
	}
	return time.Since(t) >= refreshInterval
}

func MaybeRefresh(ctx context.Context) error {
	_ = ctx
	return fmt.Errorf("automatic benchmark score refresh is disabled: assign role scores manually")
	/*
		st, err := LoadStore()
		if err != nil {
			st, err = loadEmbedded()
			if err != nil {
				return err
			}
		}
		if !NeedsRefresh(st) {
			return nil
		}
		if key := ResolveAAAPIKey(); key != "" {
			if err := RefreshFromAA(ctx, BuildOptions{APIKey: key}); err == nil {
				return nil
			}
		}
		return RefreshFromRelease(ctx)
	*/
}

func RefreshFromAA(ctx context.Context, opts BuildOptions) error {
	_ = ctx
	_ = opts
	return fmt.Errorf("automatic benchmark score refresh is disabled: assign role scores manually")
	/*
		if ctx == nil {
			ctx = context.Background()
		}
		cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		scores, manifest, err := BuildScores(cctx, opts)
		if err != nil {
			return err
		}
		if err := WriteStore(scores, manifest); err != nil {
			return err
		}
		logging.Log(logging.INFO_LOG_LEVEL, "benchmark scores refreshed from Artificial Analysis", logging.LogOptions{Params: map[string]any{"models": len(scores.Models), "sources": scores.Sources}})
		return nil
	*/
}

func RefreshFromRelease(ctx context.Context) error {
	_ = ctx
	return fmt.Errorf("automatic benchmark score refresh is disabled: assign role scores manually")
	/*
		if ctx == nil {
			ctx = context.Background()
		}
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		rel, err := fetchLatestRelease(cctx)
		if err != nil {
			return err
		}
		scoresURL, manifestURL := "", ""
		for _, a := range rel.Assets {
			switch strings.TrimSpace(a.Name) {
			case scoresAssetName:
				scoresURL = strings.TrimSpace(a.BrowserDownloadURL)
			case manifestAssetName:
				manifestURL = strings.TrimSpace(a.BrowserDownloadURL)
			}
		}
		if scoresURL == "" {
			return fmt.Errorf("release %q missing %s asset", rel.TagName, scoresAssetName)
		}
		scoresData, err := httpDownloadAsset(cctx, scoresURL)
		if err != nil {
			return err
		}
		scores, err := ParseScoresJSON(scoresData)
		if err != nil {
			return err
		}
		if err := ValidateScoresFile(scores); err != nil {
			return err
		}
		var manifest Manifest
		if manifestURL != "" {
			manifestData, mErr := httpDownloadAsset(cctx, manifestURL)
			if mErr == nil {
				manifest, _ = ParseManifestJSON(manifestData)
			}
		}
		if err := VerifyScoresSHA(scoresData, manifest); err != nil {
			return err
		}
		if strings.TrimSpace(manifest.GeneratedAt) == "" {
			manifest.GeneratedAt = strings.TrimSpace(scores.GeneratedAt)
		}
		if err := WriteStore(scores, manifest); err != nil {
			return err
		}
		logging.Log(logging.INFO_LOG_LEVEL, "benchmark scores refreshed", logging.LogOptions{Params: map[string]any{"tag": rel.TagName}})
		return nil
	*/
}

func fetchLatestRelease(ctx context.Context) (releaseAssetsJSON, error) {
	var out releaseAssetsJSON
	resp, err := httpGetRelease(ctx, "https://api.github.com/repos/SAPPHIR3-ROS3/Solomon/releases/latest")
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("github releases API: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, err
	}
	if strings.TrimSpace(out.TagName) == "" {
		return out, fmt.Errorf("empty release tag")
	}
	return out, nil
}

func LastRefreshMarkerPath() (string, error) {
	dir, err := ExtraDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".last_refresh"), nil
}

func TouchLastRefresh() error {
	p, err := LastRefreshMarkerPath()
	if err != nil {
		return err
	}
	if err := EnsureExtraDir(); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(time.Now().UTC().Format(time.RFC3339)), 0o600)
}

func SyncEmbeddedToDiskIfMissing() error {
	emb, err := loadEmbedded()
	if err != nil {
		return err
	}
	disk, diskErr := loadFromDisk()
	if diskErr != nil {
		return WriteStore(emb.Scores, emb.Manifest)
	}
	td, okd := ManifestTime(disk.Manifest)
	te, oke := ManifestTime(emb.Manifest)
	if !okd || (oke && te.After(td)) || len(emb.Scores.Models) > len(disk.Scores.Models) {
		return WriteStore(emb.Scores, emb.Manifest)
	}
	return nil
}

func ReleaseTagFromUpdater() string {
	res := updater.Check(context.Background(), "")
	if res.Err != nil {
		return ""
	}
	return strings.TrimSpace(res.LatestTag)
}
