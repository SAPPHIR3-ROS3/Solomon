package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands/connect"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/providerui"
)

const modelCatalogTTL = time.Minute

type modelAPI struct {
	mu        sync.Mutex
	catalog   apiModelCatalog
	catalogAt time.Time
}

type apiModelChoice struct {
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

type apiProviderCatalog struct {
	Complete         bool     `json:"complete"`
	Disabled         []string `json:"disabled,omitempty"`
	Models           []string `json:"models"`
	Provider         string   `json:"provider"`
	SupportsFastMode bool     `json:"supportsFastMode"`
}

type apiModelCatalog struct {
	Current   apiModelChoice       `json:"current"`
	Providers []apiProviderCatalog `json:"providers"`
	Recent    []apiModelChoice     `json:"recent"`
}

type apiModelVisibility struct {
	Enabled  bool   `json:"enabled"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

type apiQuotaBar struct {
	Label   string  `json:"label"`
	Percent float64 `json:"percent"`
}

type apiProviderQuota struct {
	Provider string        `json:"provider"`
	Error    string        `json:"error,omitempty"`
	Bars     []apiQuotaBar `json:"bars,omitempty"`
}

type apiProviderQuotas struct {
	Providers []apiProviderQuota `json:"providers"`
}

func newModelAPI() *modelAPI { return &modelAPI{} }

func (a *modelAPI) handleCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	forceRefresh := r.URL.Query().Get("refresh") == "1"
	if forceRefresh {
		codex.ResolveClientVersion(r.Context(), true)
	}
	catalog, err := a.loadCatalog(forceRefresh)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	// Visibility may have changed while provider requests were running, or
	// while the model catalog was cached. Always overlay the latest preferences.
	visibility, loadErr := config.ReadModelVisibility()
	if loadErr != nil {
		writeAPIError(w, http.StatusInternalServerError, loadErr)
		return
	}
	providers := append([]apiProviderCatalog(nil), catalog.Providers...)
	for i := range providers {
		providers[i].Disabled = config.HiddenModelIDs(visibility, providers[i].Provider, providers[i].Models)
	}
	catalog.Providers = providers
	writeJSON(w, http.StatusOK, catalog)
}

func (a *modelAPI) handleCurrent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		Model    string `json:"model"`
		Provider string `json:"provider"`
	}
	if err := decodeJSONBody(w, r, &request, 8192); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	request.Provider = strings.TrimSpace(request.Provider)
	request.Model = strings.TrimSpace(request.Model)
	if request.Provider == "" || request.Model == "" {
		writeAPIError(w, http.StatusBadRequest, errors.New("provider and model are required"))
		return
	}
	if _, err := config.UpdateCurrentModel(request.Provider, request.Model); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	a.invalidate()
	writeJSON(w, http.StatusOK, apiModelChoice{Model: request.Model, Provider: request.Provider})
}

func (a *modelAPI) handleVisibility(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		Enabled  *bool  `json:"enabled"`
		Model    string `json:"model"`
		Provider string `json:"provider"`
	}
	if err := decodeJSONBody(w, r, &request, 8192); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	request.Provider = strings.TrimSpace(request.Provider)
	request.Model = strings.TrimSpace(request.Model)
	if request.Enabled == nil || request.Provider == "" || request.Model == "" {
		writeAPIError(w, http.StatusBadRequest, errors.New("provider, model and enabled are required"))
		return
	}
	if err := config.UpdateModelVisibility(request.Provider, request.Model, *request.Enabled); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	a.invalidate()
	writeJSON(w, http.StatusOK, apiModelVisibility{Enabled: *request.Enabled, Model: request.Model, Provider: request.Provider})
}

func (a *modelAPI) handleConnectProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		APIKey  string `json:"apiKey"`
		BaseURL string `json:"baseURL"`
		Kind    int    `json:"kind"`
		Name    string `json:"name"`
	}
	if err := decodeJSONBody(w, r, &request, 32<<10); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	result, err := providerui.Connect(r.Context(), providerui.ConnectRequest{
		APIKey: request.APIKey, BaseURL: request.BaseURL, Kind: request.Kind, Name: request.Name,
	})
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	a.invalidate()
	writeJSON(w, http.StatusOK, apiModelChoice{Model: result.CurrentModel, Provider: result.CurrentProvider})
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
		if !p.IsChatGPTSub() && !p.IsClaudeSub() && !p.IsCursorAPI() {
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
		return fetchQuotaJSON(ctx, []string{"https://chatgpt.com/backend-api/usage", "https://chatgpt.com/backend-api/subscriptions"}, bearer, p.OAuthAccountID, "chatgpt", nil)
	case p.IsClaudeSub():
		return fetchQuotaJSON(ctx, []string{"https://api.anthropic.com/api/oauth/usage", "https://api.anthropic.com/v1/organizations/usage"}, bearer, "", "claude", claudeQuotaHeaders(bearer))
	case p.IsCursorAPI():
		return fetchQuotaJSON(ctx, []string{"https://api2.cursor.sh/auth/usage", "https://www.cursor.com/api/usage"}, bearer, "", "cursor", map[string]string{"Authorization": "Bearer " + bearer})
	default:
		return apiProviderQuota{}
	}
}

func claudeQuotaHeaders(bearer string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + bearer, "anthropic-version": "2023-06-01", "anthropic-beta": "oauth-2025-04-20"}
}

func fetchQuotaJSON(ctx context.Context, urls []string, bearer, accountID, kind string, extra map[string]string) apiProviderQuota {
	var last string
	for _, rawURL := range urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			last = err.Error()
			continue
		}
		req.Header.Set("Accept", "application/json")
		if extra != nil {
			for k, v := range extra {
				req.Header.Set(k, v)
			}
		} else {
			req.Header.Set("Authorization", "Bearer "+bearer)
			if accountID != "" {
				req.Header.Set("chatgpt-account-id", accountID)
			}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			last = err.Error()
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
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

func quotaBarsFromPayload(payload any, kind string) []apiQuotaBar {
	if kind == "cursor" {
		first := firstQuotaPercent(payload, []string{"first_party", "firstparty", "included", "plan", "premium"})
		third := firstQuotaPercent(payload, []string{"third_party", "thirdparty", "ondemand", "on_demand", "extra", "usage_based"})
		var bars []apiQuotaBar
		if first >= 0 {
			bars = append(bars, apiQuotaBar{Label: "First-party models", Percent: first})
		}
		if third >= 0 {
			bars = append(bars, apiQuotaBar{Label: "Third-party models", Percent: third})
		}
		if len(bars) > 0 {
			return bars
		}
	}
	if pct := firstQuotaPercent(payload, nil); pct >= 0 {
		return []apiQuotaBar{{Label: "Plan usage", Percent: pct}}
	}
	return nil
}

func firstQuotaPercent(v any, keys []string) float64 {
	switch node := v.(type) {
	case map[string]any:
		if len(keys) == 0 {
			if pct := percentFromMap(node); pct >= 0 {
				return pct
			}
		}
		for k, child := range node {
			low := strings.ToLower(k)
			if len(keys) == 0 || keyHasAny(low, keys) {
				if pct := firstQuotaPercent(child, nil); pct >= 0 {
					return pct
				}
				if pct := percentFromMap(asMap(child)); pct >= 0 {
					return pct
				}
			}
			if pct := firstQuotaPercent(child, keys); pct >= 0 {
				return pct
			}
		}
	case []any:
		for _, child := range node {
			if pct := firstQuotaPercent(child, keys); pct >= 0 {
				return pct
			}
		}
	}
	return -1
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func keyHasAny(low string, keys []string) bool {
	for _, key := range keys {
		if strings.Contains(low, key) {
			return true
		}
	}
	return false
}

func percentFromMap(m map[string]any) float64 {
	if m == nil {
		return -1
	}
	if pct, ok := numberField(m, "percent", "percentage", "pct", "used_percent", "usage_percent"); ok {
		return clampPercent(pct)
	}
	used, hasUsed := numberField(m, "used", "used_usd", "requests_used", "tokens_used", "consumed")
	limit, hasLimit := numberField(m, "limit", "limit_usd", "requests_limit", "tokens_limit", "allotment", "cap")
	if hasUsed && hasLimit && limit > 0 {
		return clampPercent(used / limit * 100)
	}
	rem, hasRem := numberField(m, "remaining", "left")
	if hasRem && hasLimit && limit > 0 {
		return clampPercent((limit - rem) / limit * 100)
	}
	return -1
}

func numberField(m map[string]any, names ...string) (float64, bool) {
	for k, v := range m {
		low := strings.ToLower(k)
		for _, name := range names {
			if low == name {
				switch n := v.(type) {
				case float64:
					return n, true
				case json.Number:
					f, err := n.Float64()
					return f, err == nil
				}
			}
		}
	}
	return 0, false
}

func clampPercent(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
func (a *modelAPI) invalidate() {
	a.mu.Lock()
	a.catalogAt = time.Time{}
	a.mu.Unlock()
}

func (a *modelAPI) loadCatalog(forceRefresh bool) (apiModelCatalog, error) {
	a.mu.Lock()
	if !forceRefresh && !a.catalogAt.IsZero() && time.Since(a.catalogAt) < modelCatalogTTL {
		catalog := a.catalog
		a.mu.Unlock()
		return catalog, nil
	}
	a.mu.Unlock()

	cfg, err := config.Load()
	if err != nil {
		return apiModelCatalog{}, err
	}
	catalog := apiModelCatalog{
		Current:   apiModelChoice{Model: strings.TrimSpace(cfg.Current.Model), Provider: strings.TrimSpace(cfg.Current.Provider)},
		Providers: []apiProviderCatalog{},
		Recent:    []apiModelChoice{},
	}
	for _, recent := range config.RecentModelUseEntries(cfg, cfg.Current.Provider) {
		if recent.Provider == config.ProviderNameClaudeSub {
			continue
		}
		catalog.Recent = append(catalog.Recent, apiModelChoice{Model: recent.Model, Provider: recent.Provider})
	}

	providers := config.ProviderList(cfg)
	filtered := make([]config.Provider, 0, len(providers))
	for _, provider := range providers {
		if provider.Name != config.ProviderNameClaudeSub {
			filtered = append(filtered, provider)
		}
	}
	catalog.Providers = make([]apiProviderCatalog, len(filtered))
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(filtered))
	for index := range filtered {
		index := index
		provider := filtered[index]
		go func() {
			defer waitGroup.Done()
			ids, listErr := connect.ListModelsForProviderAll(ctx, cfg, &provider)
			ids = uniqueModelIDs(ids)
			liveCatalog := listErr == nil && len(ids) > 0
			if len(ids) == 0 {
				ids = uniqueModelIDs(cfg.RecentModels[provider.Name])
			}
			if provider.Name == cfg.Current.Provider {
				ids = ensureModelFirst(ids, cfg.Current.Model)
			}
			catalog.Providers[index] = apiProviderCatalog{
				Complete:         liveCatalog,
				Disabled:         config.HiddenModelIDs(cfg, provider.Name, ids),
				Models:           ids,
				Provider:         provider.Name,
				SupportsFastMode: config.FastModeSupportedByProvider(&provider),
			}
		}()
	}
	waitGroup.Wait()
	a.mu.Lock()
	a.catalog = catalog
	a.catalogAt = time.Now()
	a.mu.Unlock()
	return catalog, nil
}

func uniqueModelIDs(ids []string) []string {
	result := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	sort.Strings(result)
	return result
}

func ensureModelFirst(ids []string, current string) []string {
	current = strings.TrimSpace(current)
	if current == "" {
		return ids
	}
	result := []string{current}
	for _, id := range ids {
		if id != current {
			result = append(result, id)
		}
	}
	return result
}
