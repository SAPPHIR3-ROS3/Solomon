package modelsapi

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/anthropic/claude"
	sdk "github.com/anthropics/anthropic-sdk-go"
)

var claudeSubPriorityKeywords = []string{"mythos", "fable", "opus", "sonnet", "haiku"}

func OrderClaudeSubModelIDs(ids []string) []string {
	seen := map[string]bool{}
	var all []string
	for _, raw := range ids {
		raw = strings.TrimSpace(raw)
		if raw == "" || seen[raw] {
			continue
		}
		seen[raw] = true
		all = append(all, raw)
	}
	claimed := map[string]bool{}
	var out []string
	for _, kw := range claudeSubPriorityKeywords {
		id, ok := bestClaudeIDMatching(all, kw, claimed)
		if !ok {
			continue
		}
		out = append(out, id)
		claimed[id] = true
	}
	var rest []string
	for _, id := range all {
		if claimed[id] {
			continue
		}
		rest = append(rest, id)
	}
	sort.Slice(rest, func(i, j int) bool {
		if c := compareIntSlices(modelVersionKey(rest[i]), modelVersionKey(rest[j])); c != 0 {
			return c > 0
		}
		return rest[i] < rest[j]
	})
	return append(out, rest...)
}

func bestClaudeIDMatching(ids []string, keyword string, claimed map[string]bool) (string, bool) {
	var best string
	var bestVer []int
	found := false
	for _, id := range ids {
		if claimed[id] {
			continue
		}
		m := strings.ToLower(id)
		if !strings.Contains(m, keyword) {
			continue
		}
		ver := modelVersionKey(m)
		if !found || compareIntSlices(ver, bestVer) > 0 || (compareIntSlices(ver, bestVer) == 0 && id < best) {
			best = id
			bestVer = ver
			found = true
		}
	}
	return best, found
}

func modelVersionKey(model string) []int {
	m := strings.ToLower(strings.TrimSpace(model))
	var key []int
	i := 0
	for i < len(m) {
		if m[i] < '0' || m[i] > '9' {
			i++
			continue
		}
		j := i
		for j < len(m) && m[j] >= '0' && m[j] <= '9' {
			j++
		}
		n, _ := strconv.Atoi(m[i:j])
		key = append(key, n)
		i = j
	}
	return key
}

func ListAnthropic(baseURL, token string, oauthBearer bool) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return ListAnthropicWithClient(ctx, baseURL, token, oauthBearer, nil)
}

// ListAnthropicWithClient uses the same SDK authentication and transport as inference.
func ListAnthropicWithClient(ctx context.Context, baseURL, token string, oauthBearer bool, httpClient *http.Client) ([]string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("empty anthropic token")
	}
	client := claude.NewClient(baseURL, token, oauthBearer, httpClient)
	params := sdk.ModelListParams{Limit: sdk.Int(1000)}
	var out []string
	for page := 0; page < 32; page++ {
		result, err := client.Models.List(ctx, params)
		if err != nil {
			return nil, claude.SDKError(err)
		}
		for _, model := range result.Data {
			if id := strings.TrimSpace(model.ID); id != "" {
				out = append(out, id)
			}
		}
		if !result.HasMore || result.LastID == "" {
			break
		}
		if params.AfterID.Value == result.LastID {
			return nil, fmt.Errorf("anthropic models API: pagination did not advance")
		}
		params.AfterID = sdk.String(result.LastID)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("anthropic models API: empty model list")
	}
	return out, nil
}

var anthropicFlagshipIDRE = regexp.MustCompile(`(?i)^claude-(mythos|fable|opus|sonnet|haiku)-(.+)$`)

type anthropicLine int

const (
	anthropicLineMythos anthropicLine = iota
	anthropicLineFable
	anthropicLineOpus
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
	order := []anthropicLine{anthropicLineMythos, anthropicLineFable, anthropicLineOpus, anthropicLineSonnet, anthropicLineHaiku}
	out := make([]string, 0, 5)
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
	case "mythos":
		line = anthropicLineMythos
	case "fable":
		line = anthropicLineFable
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
