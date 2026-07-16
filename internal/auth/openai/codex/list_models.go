package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

type codexModelEntry struct {
	Slug       string `json:"slug"`
	Visibility string `json:"visibility"`
	Priority   int    `json:"priority"`
}

type codexModelsResponse struct {
	Models []codexModelEntry `json:"models"`
}

func ListModels(ctx context.Context, bearer, accountID string) ([]string, error) {
	bearer = strings.TrimSpace(bearer)
	if bearer == "" {
		return nil, fmt.Errorf("ChatGPT Sub: missing OAuth access token")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	url := ChatGPTSubAPIBase + "/models?client_version=" + ClientVersion
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	applyCodexUpstreamHeaders(req, bearer, accountID)
	req.Header.Set("accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ChatGPT Sub models API: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var envelope codexModelsResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("ChatGPT Sub models API: decode: %w", err)
	}
	filtered := make([]codexModelEntry, 0, len(envelope.Models))
	for _, m := range envelope.Models {
		slug := strings.TrimSpace(m.Slug)
		if slug == "" {
			continue
		}
		if vis := strings.TrimSpace(strings.ToLower(m.Visibility)); vis == "hide" || vis == "hidden" {
			continue
		}
		m.Slug = slug
		filtered = append(filtered, m)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Priority != filtered[j].Priority {
			return filtered[i].Priority > filtered[j].Priority
		}
		return filtered[i].Slug < filtered[j].Slug
	})
	out := make([]string, 0, len(filtered))
	for _, m := range filtered {
		out = append(out, m.Slug)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("ChatGPT Sub models API: empty model list")
	}
	return out, nil
}
