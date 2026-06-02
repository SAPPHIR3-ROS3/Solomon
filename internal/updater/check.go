package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	githubOwner = "SAPPHIR3-ROS3"
	githubRepo  = "Solomon"
)

var latestReleaseAPI = "https://api.github.com/repos/SAPPHIR3-ROS3/Solomon/releases/latest"

func SetLatestReleaseAPIURL(url string) func() {
	prev := latestReleaseAPI
	latestReleaseAPI = url
	return func() { latestReleaseAPI = prev }
}

type CheckResult struct {
	Current   string
	LatestTag string
	Newer     bool
	Err       error
}

type Notice struct {
	Current string
	Latest  string
}

func (r CheckResult) Notice() *Notice {
	if r.Err != nil || !r.Newer || strings.TrimSpace(r.LatestTag) == "" {
		return nil
	}
	return &Notice{Current: r.Current, Latest: r.LatestTag}
}

type releaseJSON struct {
	TagName string `json:"tag_name"`
}

var httpGetLatest = func(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "solomon-updater")
	return http.DefaultClient.Do(req)
}

func Check(ctx context.Context, currentVersion string) CheckResult {
	current := strings.TrimSpace(currentVersion)
	res := CheckResult{Current: current}
	if ctx == nil {
		ctx = context.Background()
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	resp, err := httpGetLatest(cctx, latestReleaseAPI)
	if err != nil {
		res.Err = err
		return res
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		res.Err = fmt.Errorf("github releases API: %s", resp.Status)
		return res
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		res.Err = err
		return res
	}
	var rel releaseJSON
	if err := json.Unmarshal(body, &rel); err != nil {
		res.Err = err
		return res
	}
	tag := strings.TrimSpace(rel.TagName)
	if tag == "" {
		res.Err = fmt.Errorf("empty release tag from %s/%s", githubOwner, githubRepo)
		return res
	}
	res.LatestTag = tag
	res.Newer = IsNewerRelease(tag, current)
	return res
}
