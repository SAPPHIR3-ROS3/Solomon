package modelsapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/claudecode"
)

const anthropicAPIVersion = "2023-06-01"

const anthropicOAuthBeta = "claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14"

func CuratedAnthropicModels() []string {
	return []string{
		"claude-opus-4-8",
		"claude-sonnet-4-6",
		"claude-opus-4-6",
		"claude-sonnet-4-5-20250929",
		"claude-haiku-4-5-20251001",
	}
}

func CuratedClaudeSubModels() []string {
	return PickAnthropicFlagshipModels(CuratedAnthropicModels())
}

func anthropicModelsURL(base string) string {
	base = strings.TrimSuffix(strings.TrimSpace(base), "/")
	if strings.HasSuffix(base, "/v1/models") {
		return base
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/models"
	}
	return base + "/v1/models"
}

type anthropicListResp struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
	HasMore bool   `json:"has_more"`
	LastID  string `json:"last_id"`
}

func ListAnthropic(baseURL, token string, oauthBearer bool) ([]string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("empty anthropic token")
	}
	cli := &http.Client{Timeout: 60 * time.Second}
	var out []string
	afterID := ""
	for page := 0; page < 32; page++ {
		u := anthropicModelsURL(baseURL)
		q := url.Values{}
		q.Set("limit", "1000")
		if afterID != "" {
			q.Set("after_id", afterID)
		}
		u += "?" + q.Encode()
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("anthropic-version", anthropicAPIVersion)
		if oauthBearer {
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("anthropic-beta", anthropicOAuthBeta)
			req.Header.Set("Accept", "application/json")
			req.Header.Set("user-agent", "claude-cli/"+claudecode.Version())
			req.Header.Set("x-app", "cli")
			req.Header.Set("anthropic-dangerous-direct-browser-access", "true")
		} else {
			req.Header.Set("x-api-key", token)
		}
		resp, err := cli.Do(req)
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("anthropic models API: %s: %s", resp.Status, strings.TrimSpace(string(b)))
		}
		var lr anthropicListResp
		if err := json.Unmarshal(b, &lr); err != nil {
			return nil, err
		}
		for _, e := range lr.Data {
			if id := strings.TrimSpace(e.ID); id != "" {
				out = append(out, id)
			}
		}
		if !lr.HasMore || lr.LastID == "" {
			break
		}
		afterID = lr.LastID
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("anthropic models API: empty model list")
	}
	return out, nil
}

var anthropicFlagshipIDRE = regexp.MustCompile(`(?i)^claude-(opus|sonnet|haiku)-(.+)$`)

type anthropicLine int

const (
	anthropicLineOpus anthropicLine = iota
	anthropicLineSonnet
	anthropicLineHaiku
)

type anthropicModelRank struct {
	ver  []int
	date int
	ok   bool
}

func PickAnthropicFlagshipModels(ids []string) []string {
	best := map[anthropicLine]string{}
	bestRank := map[anthropicLine]anthropicModelRank{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || skipAnthropicFlagshipCandidate(id) {
			continue
		}
		line, rank, ok := rankAnthropicModelID(id)
		if !ok || !rank.ok {
			continue
		}
		prev, has := bestRank[line]
		if !has || anthropicRankBetter(rank, prev) {
			best[line] = id
			bestRank[line] = rank
		}
	}
	order := []anthropicLine{anthropicLineOpus, anthropicLineSonnet, anthropicLineHaiku}
	out := make([]string, 0, 3)
	for _, line := range order {
		if id := best[line]; id != "" {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return append([]string(nil), ids...)
	}
	return out
}

func skipAnthropicFlagshipCandidate(id string) bool {
	m := strings.ToLower(id)
	for _, bad := range []string{"thinking", "instant", "legacy"} {
		if strings.Contains(m, bad) {
			return true
		}
	}
	return false
}

func rankAnthropicModelID(id string) (anthropicLine, anthropicModelRank, bool) {
	m := strings.ToLower(strings.TrimSpace(id))
	sub := anthropicFlagshipIDRE.FindStringSubmatch(m)
	if len(sub) != 3 {
		return 0, anthropicModelRank{}, false
	}
	var line anthropicLine
	switch sub[1] {
	case "opus":
		line = anthropicLineOpus
	case "sonnet":
		line = anthropicLineSonnet
	case "haiku":
		line = anthropicLineHaiku
	default:
		return 0, anthropicModelRank{}, false
	}
	ver, date, ok := parseAnthropicVersionSuffix(sub[2])
	if !ok || len(ver) == 0 {
		return line, anthropicModelRank{}, true
	}
	return line, anthropicModelRank{ver: ver, date: date, ok: true}, true
}

func parseAnthropicVersionSuffix(suffix string) ([]int, int, bool) {
	suffix = strings.TrimSpace(suffix)
	if suffix == "" {
		return nil, 0, false
	}
	parts := strings.Split(suffix, "-")
	date := 0
	if len(parts) > 0 {
		if d, err := strconv.Atoi(parts[len(parts)-1]); err == nil && len(parts[len(parts)-1]) == 8 {
			date = d
			parts = parts[:len(parts)-1]
		}
	}
	var ver []int
	for _, p := range parts {
		if p == "" {
			continue
		}
		for _, seg := range strings.Split(p, ".") {
			if seg == "" {
				continue
			}
			n, err := strconv.Atoi(seg)
			if err != nil {
				return nil, 0, false
			}
			ver = append(ver, n)
		}
	}
	if len(ver) == 0 {
		return nil, date, date > 0
	}
	return ver, date, true
}

func anthropicRankBetter(a, b anthropicModelRank) bool {
	if c := compareIntSlices(a.ver, b.ver); c != 0 {
		return c > 0
	}
	if a.date == 0 && b.date != 0 {
		return true
	}
	if a.date != 0 && b.date == 0 {
		return false
	}
	return a.date > b.date
}

func compareIntSlices(a, b []int) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		var xa, xb int
		if i < len(a) {
			xa = a[i]
		}
		if i < len(b) {
			xb = b[i]
		}
		if xa != xb {
			return xa - xb
		}
	}
	return 0
}
