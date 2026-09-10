package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

type apiQuotaBar struct {
	Label   string  `json:"label"`
	Percent float64 `json:"percent"`
	Detail  string  `json:"detail,omitempty"`
}

type apiProviderQuota struct {
	Provider string        `json:"provider"`
	Error    string        `json:"error,omitempty"`
	Bars     []apiQuotaBar `json:"bars,omitempty"`
}

var providerQuotaHTTPClient = &http.Client{Timeout: 15 * time.Second}

type apiProviderQuotas struct {
	Providers []apiProviderQuota `json:"providers"`
}

func (a *modelAPI) handleQuotas(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	cfg, err := config.Load()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	out := apiProviderQuotas{Providers: []apiProviderQuota{}}
	for _, provider := range config.ProviderList(cfg) {
		p := provider
		if !p.IsChatGPTSub() && !p.IsClaudeSub() && !p.IsCursorAPI() && !isOpenRouterProvider(&p) {
			continue
		}
		q := loadProviderQuota(r.Context(), cfg, &p)
		q.Provider = p.Name
		out.Providers = append(out.Providers, q)
	}
	writeJSON(w, http.StatusOK, out)
}

func loadProviderQuota(ctx context.Context, cfg *config.Root, p *config.Provider) apiProviderQuota {
	bearer, err := config.ResolveProviderBearer(ctx, cfg, p)
	if err != nil {
		return apiProviderQuota{Error: err.Error()}
	}
	switch {
	case p.IsChatGPTSub():
		return fetchQuotaJSON(ctx, []string{"https://chatgpt.com/backend-api/wham/usage"}, bearer, p.OAuthAccountID, "chatgpt", nil)
	case p.IsClaudeSub():
		return fetchQuotaJSON(ctx, []string{"https://api.anthropic.com/api/oauth/usage"}, bearer, "", "claude", claudeQuotaHeaders(bearer))
	case p.IsCursorAPI():
		return fetchCursorQuota(ctx, bearer)
	case isOpenRouterProvider(p):
		return fetchQuotaJSON(ctx, []string{"https://openrouter.ai/api/v1/credits"}, bearer, "", "openrouter", nil)
	default:
		return apiProviderQuota{}
	}
}

func claudeQuotaHeaders(bearer string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + bearer, "anthropic-version": "2023-06-01", "anthropic-beta": "oauth-2025-04-20"}
}

func fetchQuotaJSON(ctx context.Context, urls []string, bearer, accountID, kind string, extra map[string]string) apiProviderQuota {
	if ctx == nil {
		ctx = context.Background()
	}
	var last string
	for _, rawURL := range urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			last = err.Error()
			continue
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", quotaUserAgent(kind))
		if kind == "chatgpt" {
			req.Header.Set("Originator", codex.Originator)
			req.Header.Set("Version", codex.ResolveClientVersion(ctx, false))
			req.Header.Set("X-Codex-Beta-Features", "multi_agent,apps,prevent_idle_sleep")
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		if accountID != "" {
			req.Header.Set("ChatGPT-Account-Id", accountID)
		}
		for key, value := range extra {
			req.Header.Set(key, value)
		}
		resp, err := providerQuotaHTTPClient.Do(req)
		if err != nil {
			last = err.Error()
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			last = readErr.Error()
			continue
		}
		if resp.StatusCode != http.StatusOK {
			last = fmt.Sprintf("%s: %s", rawURL, resp.Status)
			continue
		}
		var payload any
		if err := json.Unmarshal(body, &payload); err != nil {
			last = err.Error()
			continue
		}
		bars := quotaBarsFromPayload(payload, kind)
		if len(bars) > 0 {
			return apiProviderQuota{Bars: bars}
		}
		last = "quota fields missing"
	}
	return apiProviderQuota{Error: last}
}

func fetchCursorQuota(ctx context.Context, bearer string) apiProviderQuota {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(bearer)), "crsr_") {
		return apiProviderQuota{Error: "Cursor usage requires a web session token; Cursor API keys do not expose dashboard quota data"}
	}
	return fetchQuotaJSON(ctx, []string{
		"https://api2.cursor.sh/auth/usage",
		"https://cursor.com/api/usage-summary",
		"https://www.cursor.com/api/usage-summary",
	}, bearer, "", "cursor", map[string]string{
		"Origin":  "https://cursor.com",
		"Referer": "https://cursor.com/dashboard/usage",
	})
}

func quotaUserAgent(kind string) string {
	if kind == "chatgpt" {
		return "codex_cli_rs/" + codex.ClientVersion + " (Ubuntu 22.04.0; x86_64) WindowsTerminal"
	}
	return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/131.0.0.0 Safari/537.36"
}

func isOpenRouterProvider(p *config.Provider) bool {
	if p == nil {
		return false
	}
	value := strings.ToLower(strings.TrimSpace(p.Name + " " + p.BaseURL))
	return strings.Contains(value, "openrouter")
}
