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

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/roles"
)

const aaBaseURL = "https://artificialanalysis.ai/api/v2"

type aaModel struct {
	Slug         string
	Name         string
	ReleaseDate  string
	Deprecated   bool
	Evals        map[string]float64
	ListPrice    float64
	IntelCost    float64
	CodingCost   float64
	TokPerSec    float64
	TTFT         float64
	Intelligence float64
	CodingIndex  float64
	MathIndex    float64
}

type aaListResponse struct {
	Pagination struct {
		Page       int  `json:"page"`
		TotalPages int  `json:"total_pages"`
		HasMore    bool `json:"has_more"`
	} `json:"pagination"`
	Data []json.RawMessage `json:"data"`
}

var httpGetAA = func(ctx context.Context, url, apiKey string) ([]byte, error) {
	if HTTPGetAAForTest != nil {
		return HTTPGetAAForTest(ctx, url, apiKey)
	}
	return httpGetAALive(ctx, url, apiKey)
}

var HTTPGetAAForTest func(ctx context.Context, url, apiKey string) ([]byte, error)

var httpGetAALive = func(ctx context.Context, url, apiKey string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("User-Agent", "solomon-benchmarks")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("artificial analysis %s: %s (%s)", url, resp.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func ResolveAAAPIKey() string {
	if key := strings.TrimSpace(os.Getenv("AA_API_KEY")); key != "" {
		return key
	}
	if key := strings.TrimSpace(os.Getenv("ARTIFICIAL_ANALYSIS_API_KEY")); key != "" {
		return key
	}
	home, err := paths.SolomonHome()
	if err != nil {
		return ""
	}
	return loadDotEnvKey(filepath.Join(home, ".env"), "AA_API_KEY", "ARTIFICIAL_ANALYSIS_API_KEY")
}

func loadDotEnvKey(path string, keys ...string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	vals := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		vals[key] = val
	}
	for _, key := range keys {
		if v := strings.TrimSpace(vals[key]); v != "" {
			return v
		}
	}
	return ""
}

func fetchAAModels(ctx context.Context, apiKey string) ([]aaModel, string, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, "", fmt.Errorf("AA_API_KEY is required to refresh benchmark scores from Artificial Analysis")
	}
	proURL := aaBaseURL + "/language/models?page=%d&page_size=100"
	freeURL := aaBaseURL + "/language/models/free?page=%d&page_size=100"
	var (
		models []aaModel
		tier   string
		err    error
	)
	models, tier, err = fetchAAPages(ctx, apiKey, proURL)
	if err != nil {
		if isAAForbidden(err) {
			models, tier, err = fetchAAPages(ctx, apiKey, freeURL)
		}
	}
	if err != nil {
		return nil, "", err
	}
	if len(models) == 0 {
		return nil, tier, fmt.Errorf("artificial analysis returned no models")
	}
	return models, tier, nil
}

func isAAForbidden(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "403") || strings.Contains(msg, "subscription")
}

func fetchAAPages(ctx context.Context, apiKey, urlFmt string) ([]aaModel, string, error) {
	var out []aaModel
	tier := ""
	for page := 1; page <= 50; page++ {
		url := fmt.Sprintf(urlFmt, page)
		body, err := httpGetAA(ctx, url, apiKey)
		if err != nil {
			return nil, "", err
		}
		var envelope struct {
			Tier       string            `json:"tier"`
			Data       []json.RawMessage `json:"data"`
			Pagination struct {
				HasMore    bool `json:"has_more"`
				TotalPages int  `json:"total_pages"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, "", err
		}
		if tier == "" {
			tier = strings.TrimSpace(envelope.Tier)
		}
		for _, raw := range envelope.Data {
			m, ok := parseAAModel(raw)
			if ok {
				out = append(out, m)
			}
		}
		if !envelope.Pagination.HasMore && page >= envelope.Pagination.TotalPages {
			break
		}
		if len(envelope.Data) == 0 {
			break
		}
	}
	return out, tier, nil
}

func parseAAModel(raw json.RawMessage) (aaModel, bool) {
	var row struct {
		Slug        string                     `json:"slug"`
		Name        string                     `json:"name"`
		ReleaseDate string                     `json:"release_date"`
		Deprecated  bool                       `json:"deprecated"`
		Evaluations map[string]json.RawMessage `json:"evaluations"`
		Pricing     struct {
			Input  float64 `json:"price_1m_input_tokens"`
			Output float64 `json:"price_1m_output_tokens"`
		} `json:"pricing"`
		Performance struct {
			TokPerSec float64 `json:"median_output_tokens_per_second"`
			TTFT      float64 `json:"median_time_to_first_token_seconds"`
		} `json:"performance"`
		IntelCost struct {
			Total float64 `json:"total_cost"`
		} `json:"artificial_analysis_intelligence_index_cost"`
		CodingCost struct {
			Total float64 `json:"total_cost"`
		} `json:"artificial_analysis_coding_index_cost"`
	}
	if err := json.Unmarshal(raw, &row); err != nil {
		return aaModel{}, false
	}
	slug := strings.TrimSpace(row.Slug)
	if slug == "" || shouldSkipAAModel(slug, row.Name) {
		return aaModel{}, false
	}
	evals := map[string]float64{}
	for k, v := range row.Evaluations {
		if f, ok := parseJSONFloat(v); ok {
			evals[k] = f
		}
	}
	m := aaModel{
		Slug:         slug,
		Name:         strings.TrimSpace(row.Name),
		ReleaseDate:  strings.TrimSpace(row.ReleaseDate),
		Deprecated:   row.Deprecated,
		Evals:        evals,
		ListPrice:    row.Pricing.Input + row.Pricing.Output,
		IntelCost:    row.IntelCost.Total,
		CodingCost:   row.CodingCost.Total,
		TokPerSec:    row.Performance.TokPerSec,
		TTFT:         row.Performance.TTFT,
		Intelligence: evals["artificial_analysis_intelligence_index"],
		CodingIndex:  evals["artificial_analysis_coding_index"],
		MathIndex:    evals["artificial_analysis_math_index"],
	}
	return m, true
}

func parseJSONFloat(raw json.RawMessage) (float64, bool) {
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f, true
	}
	return 0, false
}

func shouldSkipAAModel(slug, name string) bool {
	s := strings.ToLower(slug + " " + name)
	for _, bad := range []string{"embed", "image", "audio", "realtime", "transcribe", "tts", "ocr", "diffusion"} {
		if strings.Contains(s, bad) {
			return true
		}
	}
	return false
}

func (m aaModel) canonicalKey() string {
	return roles.NormalizeModelID(m.Slug)
}

func (m aaModel) priorityScore() int {
	score := roles.ModelRecencyScore(m.Slug)
	if m.Intelligence > 0 {
		score += int(m.Intelligence)
	}
	if t, ok := parseReleaseDate(m.ReleaseDate); ok {
		score += int(t.Unix() / 86400)
	}
	if m.Deprecated {
		score -= 1000
	}
	return score
}

func parseReleaseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
