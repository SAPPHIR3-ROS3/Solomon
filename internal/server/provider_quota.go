package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

type apiQuotaBar struct {
	Label       string  `json:"label"`
	Percent     float64 `json:"percent"`
	Detail      string  `json:"detail,omitempty"`
	HidePercent bool    `json:"hidePercent,omitempty"`
}

type apiProviderQuota struct {
	Provider   string        `json:"provider"`
	Error      string        `json:"error,omitempty"`
	CanRelogin bool          `json:"canRelogin,omitempty"`
	Bars       []apiQuotaBar `json:"bars,omitempty"`
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
		if !p.IsChatGPTSub() && !p.IsClaudeSub() && !p.IsCursorSub() && !isOpenRouterProvider(&p) {
			continue
		}
		q := loadProviderQuota(r.Context(), cfg, &p)
		q.Provider = p.Name
		q.CanRelogin = (p.IsChatGPTSub() || p.IsClaudeSub() || p.IsCursorSub()) && strings.TrimSpace(q.Error) != ""
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
		return fetchChatGPTQuota(ctx, bearer, p.OAuthAccountID)
	case p.IsClaudeSub():
		return fetchQuotaJSON(ctx, []string{"https://api.anthropic.com/api/oauth/usage"}, bearer, "", "claude", claudeQuotaHeaders(bearer))
	case p.IsCursorSub():
		session, err := config.ResolveCursorSessionBearer(ctx, cfg, p)
		if err != nil {
			return apiProviderQuota{Error: err.Error()}
		}
		return fetchCursorQuota(ctx, session)
	case isOpenRouterProvider(p):
		return fetchQuotaJSON(ctx, []string{"https://openrouter.ai/api/v1/credits"}, bearer, "", "openrouter", nil)
	default:
		return apiProviderQuota{}
	}
}

func fetchChatGPTQuota(ctx context.Context, bearer, accountID string) apiProviderQuota {
	quota := fetchQuotaJSON(ctx, []string{"https://chatgpt.com/backend-api/wham/usage"}, bearer, accountID, "chatgpt", nil)
	credits := fetchQuotaJSON(ctx, []string{"https://chatgpt.com/backend-api/wham/rate-limit-reset-credits"}, bearer, accountID, "chatgpt-resets", nil)
	if len(credits.Bars) > 0 {
		quota.Bars = replaceOrAppendQuotaBars(quota.Bars, credits.Bars)
		if quota.Error != "" && len(quota.Bars) > 0 {
			quota.Error = ""
		}
	}
	return quota
}

func replaceOrAppendQuotaBars(bars, extra []apiQuotaBar) []apiQuotaBar {
	out := append([]apiQuotaBar{}, bars...)
	for _, bar := range extra {
		replaced := false
		for i, existing := range out {
			if existing.Label == bar.Label {
				out[i] = bar
				replaced = true
				break
			}
		}
		if !replaced {
			out = append(out, bar)
		}
	}
	return out
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
		if kind == "chatgpt" || kind == "chatgpt-resets" {
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
			last = quotaErrorFromBody(resp.Status, body)
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
	bearer = strings.TrimSpace(bearer)
	if bearer == "" || strings.HasPrefix(strings.ToLower(bearer), "crsr_") {
		return apiProviderQuota{Error: "Cursor Sub: missing session token; run /connect"}
	}
	if q := fetchCursorDashboardUsage(ctx, bearer); len(q.Bars) > 0 {
		return q
	} else if q.Error != "" {
		summary := fetchQuotaJSON(ctx, []string{
			"https://cursor.com/api/usage-summary",
			"https://www.cursor.com/api/usage-summary",
		}, "", "", "cursor", cursorQuotaHeaders(bearer))
		if len(summary.Bars) > 0 {
			return summary
		}
		if summary.Error != "" {
			return q
		}
		return summary
	}
	return fetchQuotaJSON(ctx, []string{
		"https://cursor.com/api/usage-summary",
		"https://www.cursor.com/api/usage-summary",
	}, "", "", "cursor", cursorQuotaHeaders(bearer))
}

func fetchCursorDashboardUsage(ctx context.Context, bearer string) apiProviderQuota {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api2.cursor.sh/aiserver.v1.DashboardService/GetCurrentPeriodUsage", strings.NewReader("{}"))
	if err != nil {
		return apiProviderQuota{Error: err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", quotaUserAgent("cursor"))
	resp, err := providerQuotaHTTPClient.Do(req)
	if err != nil {
		return apiProviderQuota{Error: err.Error()}
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
	if readErr != nil {
		return apiProviderQuota{Error: readErr.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		return apiProviderQuota{Error: quotaErrorFromBody(resp.Status, body)}
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return apiProviderQuota{Error: err.Error()}
	}
	bars := quotaBarsFromPayload(payload, "cursor")
	if len(bars) == 0 {
		bars = []apiQuotaBar{
			{Label: "Cursor Models", Percent: 0, Detail: "Includes Cursor Grok and Composer"},
			{Label: "Other Models", Percent: 0},
		}
	}
	return apiProviderQuota{Bars: bars}
}

func cursorQuotaHeaders(session string) map[string]string {
	return map[string]string{
		"Origin":  "https://cursor.com",
		"Referer": "https://cursor.com/dashboard/usage",
		"Cookie":  cursorWebSessionCookie(session),
	}
}

func cursorWebSessionCookie(session string) string {
	session = strings.TrimSpace(session)
	if session == "" {
		return ""
	}
	if strings.Contains(session, "::") {
		return "WorkosCursorSessionToken=" + url.QueryEscape(session)
	}
	if userID := jwtClaim(session, "sub"); userID != "" {
		return "WorkosCursorSessionToken=" + url.QueryEscape(userID+"::"+session)
	}
	return "WorkosCursorSessionToken=" + session
}

func jwtClaim(token, name string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		raw, decErr := base64.StdEncoding.DecodeString(padQuotaBase64(parts[1]))
		if decErr != nil {
			return ""
		}
		payload = raw
	}
	var claims map[string]any
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	value, _ := claims[name].(string)
	return strings.TrimSpace(value)
}

func padQuotaBase64(value string) string {
	switch len(value) % 4 {
	case 2:
		return value + "=="
	case 3:
		return value + "="
	default:
		return value
	}
}

func quotaBarsFromChatGPTResetCredits(payload any) []apiQuotaBar {
	block := resetCreditsBlock(payload)
	if block == nil {
		return nil
	}
	count, hasCount := numberField(block, "available_count", "availableCount")
	credits := childSlice(block, "credits")
	if !hasCount {
		available := 0
		for _, credit := range credits {
			if resetCreditAvailable(asMap(credit)) {
				available++
			}
		}
		if available == 0 && len(credits) == 0 {
			return nil
		}
		count = float64(available)
		hasCount = true
	}
	detail := fmt.Sprintf("%d available", int64(count))
	if int64(count) == 1 {
		detail = "1 available"
	}
	if nearest := nearestAvailableResetCreditExpiry(credits); !nearest.IsZero() {
		detail += " · nearest expires " + nearest.Local().Format("2006-01-02 15:04")
		if seconds := time.Until(nearest).Seconds(); seconds > 0 {
			detail += " (" + formatDuration(int64(seconds)) + ")"
		}
	}
	return []apiQuotaBar{{Label: "Banked resets", Detail: detail, HidePercent: true}}
}

func resetCreditsBlock(payload any) map[string]any {
	if child := mapChild(payload, "rate_limit_reset_credits"); child != nil {
		return asMap(child)
	}
	if child := mapChild(payload, "rateLimitResetCredits"); child != nil {
		return asMap(child)
	}
	m := asMap(payload)
	if m == nil {
		return nil
	}
	if _, ok := numberField(m, "available_count", "availableCount"); ok {
		return m
	}
	if mapChild(m, "credits") != nil {
		return m
	}
	return nil
}

func childSlice(m map[string]any, key string) []any {
	list, ok := mapChild(m, key).([]any)
	if !ok {
		return nil
	}
	return list
}

func resetCreditAvailable(m map[string]any) bool {
	if m == nil {
		return false
	}
	status := strings.ToLower(quotaStringField(m, "status"))
	return status == "" || status == "available"
}

func nearestAvailableResetCreditExpiry(credits []any) time.Time {
	var nearest time.Time
	now := time.Now()
	for _, credit := range credits {
		m := asMap(credit)
		if !resetCreditAvailable(m) {
			continue
		}
		expiry := parseQuotaTime(m, "expires_at", "expiresAt", "expire_at", "expireAt")
		if expiry.IsZero() || !expiry.After(now) {
			continue
		}
		if nearest.IsZero() || expiry.Before(nearest) {
			nearest = expiry
		}
	}
	return nearest
}

func parseQuotaTime(m map[string]any, names ...string) time.Time {
	if raw := quotaStringField(m, names...); raw != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			return parsed
		}
		if timestamp, err := strconv.ParseFloat(raw, 64); err == nil {
			return quotaTimestamp(timestamp)
		}
	}
	if timestamp, ok := numberField(m, names...); ok {
		return quotaTimestamp(timestamp)
	}
	return time.Time{}
}

func quotaUserAgent(kind string) string {
	if kind == "chatgpt" || kind == "chatgpt-resets" {
		return "codex_cli_rs/" + codex.ClientVersion + " (Ubuntu 22.04.0; x86_64) WindowsTerminal"
	}
	return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/131.0.0.0 Safari/537.36"
}

func quotaErrorFromBody(status string, body []byte) string {
	text := strings.TrimSpace(string(body))
	if text != "" && !strings.HasPrefix(strings.ToLower(text), "<") {
		if msg := extractQuotaErrorText(text); msg != "" {
			return msg
		}
	}
	return strings.TrimSpace(status)
}

func extractQuotaErrorText(text string) string {
	var payload any
	if json.Unmarshal([]byte(text), &payload) != nil {
		if len(text) > 240 {
			return strings.TrimSpace(text[:240])
		}
		return text
	}
	return quotaErrorFromValue(payload)
}

func quotaErrorFromValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		for _, key := range []string{"error", "message", "detail", "error_message"} {
			if msg := quotaErrorFromValue(typed[key]); msg != "" {
				return msg
			}
		}
	}
	return ""
}

func isOpenRouterProvider(p *config.Provider) bool {
	if p == nil {
		return false
	}
	value := strings.ToLower(strings.TrimSpace(p.Name + " " + p.BaseURL))
	return strings.Contains(value, "openrouter")
}
