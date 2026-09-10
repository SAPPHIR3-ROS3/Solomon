package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

const (
	githubOwner = "SAPPHIR3-ROS3"
	githubRepo  = "Solomon"
)

var latestReleaseAPI = "https://api.github.com/repos/SAPPHIR3-ROS3/Solomon/releases/latest"
var compareReleaseAPI = "https://api.github.com/repos/SAPPHIR3-ROS3/Solomon/compare"
var releaseCommitAPI = "https://api.github.com/repos/SAPPHIR3-ROS3/Solomon/commits"

func SetLatestReleaseAPIURL(url string) func() {
	prev := latestReleaseAPI
	latestReleaseAPI = url
	return func() { latestReleaseAPI = prev }
}

func SetCompareReleaseAPIURL(url string) func() {
	prev := compareReleaseAPI
	compareReleaseAPI = url
	return func() { compareReleaseAPI = prev }
}

func SetReleaseCommitAPIURL(url string) func() {
	prev := releaseCommitAPI
	releaseCommitAPI = url
	return func() { releaseCommitAPI = prev }
}

type CheckResult struct {
	Current             string
	LatestTag           string
	Newer               bool
	Err                 error
	LocalCommitRelation string
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

type compareJSON struct {
	Status string `json:"status"`
}

type commitJSON struct {
	Commit struct {
		Author struct {
			Date time.Time `json:"date"`
		} `json:"author"`
		Committer struct {
			Date time.Time `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

func githubAuthToken() string {
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

func applyGitHubAPIHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "solomon-updater")
	if tok := githubAuthToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
}

var httpGetLatest = func(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	applyGitHubAPIHeaders(req)
	return http.DefaultClient.Do(req)
}

func Check(ctx context.Context, currentVersion string) CheckResult {
	return CheckWithCommitTime(ctx, currentVersion, "", time.Time{})
}

func CheckWithCommit(ctx context.Context, currentVersion, localCommit string) CheckResult {
	return CheckWithCommitTime(ctx, currentVersion, localCommit, time.Time{})
}

func CheckWithCommitTime(ctx context.Context, currentVersion, localCommit string, localCommitTime time.Time) CheckResult {
	current := strings.TrimSpace(currentVersion)
	localCommit = strings.TrimSpace(localCommit)
	res := CheckResult{Current: current}
	if ctx == nil {
		ctx = context.Background()
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	resp, err := httpGetLatest(cctx, latestReleaseAPI)
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "updater check failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		res.Err = err
		return res
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		res.Err = fmt.Errorf("github releases API: %s", resp.Status)
		logging.Log(logging.WARNING_LOG_LEVEL, "updater check github API failed", logging.LogOptions{Params: map[string]any{"status": resp.Status}})
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
	if IsDevelopmentVersion(current) && localCommit != "" {
		relation, err := compareLocalCommit(cctx, tag, localCommit)
		res.LocalCommitRelation = relation
		if err == nil && (relation == "ahead" || relation == "identical") {
			logging.Log(logging.INFO_LOG_LEVEL, "updater local development build is not behind latest release", logging.LogOptions{Params: map[string]any{"current": current, "latest": tag, "relation": relation}})
			return res
		}
		if err != nil || relation != "behind" {
			if localCommitTime.IsZero() {
				logging.Log(logging.WARNING_LOG_LEVEL, "updater commit comparison unavailable; refusing to replace development build", logging.LogOptions{Params: map[string]any{"tag": tag, "commit": localCommit}})
				return res
			}
			latestCommitTime, timeErr := fetchReleaseCommitTime(cctx, tag)
			if timeErr != nil {
				logging.Log(logging.WARNING_LOG_LEVEL, "updater release commit timestamp unavailable; refusing to replace development build", logging.LogOptions{Params: map[string]any{"tag": tag, "commit": localCommit, "err": timeErr.Error()}})
				return res
			}
			if localCommitTime.After(latestCommitTime) {
				res.LocalCommitRelation = "ahead"
				logging.Log(logging.INFO_LOG_LEVEL, "updater local development build commit is newer than latest release", logging.LogOptions{Params: map[string]any{"current": current, "latest": tag, "relation": "ahead"}})
				return res
			}
		}
		if err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "updater commit comparison failed; using version comparison", logging.LogOptions{Params: map[string]any{"tag": tag, "commit": localCommit, "err": err.Error()}})
		}
	}
	res.Newer = IsNewerRelease(tag, current)
	if res.Newer {
		logging.Log(logging.INFO_LOG_LEVEL, "updater newer release available", logging.LogOptions{Params: map[string]any{"current": current, "latest": tag}})
	}
	return res
}

func compareLocalCommit(ctx context.Context, tag, localCommit string) (string, error) {
	compareURL := strings.TrimRight(compareReleaseAPI, "/") + "/" + url.PathEscape(tag) + "..." + url.PathEscape(localCommit)
	resp, err := httpGetLatest(ctx, compareURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github compare API: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var comparison compareJSON
	if err := json.Unmarshal(body, &comparison); err != nil {
		return "", err
	}
	status := strings.TrimSpace(strings.ToLower(comparison.Status))
	if status == "" {
		return "", fmt.Errorf("github compare API returned an empty status")
	}
	return status, nil
}

func fetchReleaseCommitTime(ctx context.Context, tag string) (time.Time, error) {
	commitURL := strings.TrimRight(releaseCommitAPI, "/") + "/" + url.PathEscape(tag)
	resp, err := httpGetLatest(ctx, commitURL)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return time.Time{}, fmt.Errorf("github commit API: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return time.Time{}, err
	}
	var commit commitJSON
	if err := json.Unmarshal(body, &commit); err != nil {
		return time.Time{}, err
	}
	if !commit.Commit.Committer.Date.IsZero() {
		return commit.Commit.Committer.Date, nil
	}
	if !commit.Commit.Author.Date.IsZero() {
		return commit.Commit.Author.Date, nil
	}
	return time.Time{}, fmt.Errorf("github commit API returned no commit timestamp")
}
