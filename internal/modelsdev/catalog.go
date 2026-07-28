// Package modelsdev reads the public models.dev catalog used to enrich model
// selectors with provider-specific capabilities.
package modelsdev

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const catalogURL = "https://models.dev/api.json"

const cacheTTL = 12 * time.Hour

type Model struct {
	ID         string `json:"id"`
	Modalities struct {
		Input []string `json:"input"`
	} `json:"modalities"`
	Limit struct {
		Context int `json:"context"`
		Output  int `json:"output"`
	} `json:"limit"`
}

type Provider struct {
	API    string           `json:"api"`
	ID     string           `json:"id"`
	Models map[string]Model `json:"models"`
	Name   string           `json:"name"`
}

type Catalog map[string]Provider

var (
	cacheMu    sync.Mutex
	cached     Catalog
	cachedAt   time.Time
	fetching   chan struct{}
	httpClient = &http.Client{Timeout: 8 * time.Second}
)

// Load returns a cached catalog when available. The first caller downloads the
// public catalog; concurrent callers share that request.
func Load(ctx context.Context) (Catalog, error) {
	cacheMu.Lock()
	if len(cached) > 0 && time.Since(cachedAt) < cacheTTL {
		out := cached
		cacheMu.Unlock()
		return out, nil
	}
	if fetching != nil {
		wait := fetching
		cacheMu.Unlock()
		select {
		case <-wait:
			cacheMu.Lock()
			out := cached
			cacheMu.Unlock()
			if len(out) == 0 {
				return nil, fmt.Errorf("models.dev catalog is unavailable")
			}
			return out, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	wait := make(chan struct{})
	fetching = wait
	cacheMu.Unlock()

	catalog, err := fetch(ctx)
	cacheMu.Lock()
	if err == nil {
		cached = catalog
		cachedAt = time.Now()
	}
	out := cached
	fetching = nil
	close(wait)
	cacheMu.Unlock()
	if err != nil {
		if len(out) > 0 {
			return out, nil
		}
		return nil, err
	}
	return out, nil
}

func fetch(ctx context.Context) (Catalog, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, catalogURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download models.dev catalog: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download models.dev catalog: %s", resp.Status)
	}
	var catalog Catalog
	if err := json.NewDecoder(resp.Body).Decode(&catalog); err != nil {
		return nil, fmt.Errorf("decode models.dev catalog: %w", err)
	}
	return catalog, nil
}

// Lookup finds the exact model as served by a configured provider. It does not
// fall back to another provider because limits and supported input can differ.
func (c Catalog) Lookup(providerName, baseURL, modelID string) (Model, bool) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return Model{}, false
	}
	if normalized(providerName) == "cursorapi" {
		return c.lookupCursorModel(modelID)
	}
	for _, provider := range c {
		if !sameProvider(provider, providerName, baseURL) {
			continue
		}
		model, ok := provider.Models[modelID]
		return model, ok
	}
	return Model{}, false
}

// Cursor is a gateway, not the model's author. Its catalog mixes models from
// several labs, so derive visual capabilities from the corresponding official
// provider instead of treating Cursor as a provider of the model.
func (c Catalog) lookupCursorModel(modelID string) (Model, bool) {
	for _, providerID := range officialProvidersForCursorModel(modelID) {
		provider, ok := c[providerID]
		if !ok {
			continue
		}
		if model, ok := provider.Models[modelID]; ok {
			return model, true
		}
	}
	return Model{}, false
}

func officialProvidersForCursorModel(modelID string) []string {
	id := strings.ToLower(strings.TrimSpace(modelID))
	switch {
	case strings.HasPrefix(id, "gpt") || strings.Contains(id, "openai"):
		return []string{"openai"}
	case strings.Contains(id, "claude"):
		return []string{"anthropic"}
	case strings.Contains(id, "grok"):
		return []string{"xai"}
	case strings.Contains(id, "gemini") || strings.Contains(id, "google"):
		return []string{"google"}
	case strings.Contains(id, "glm"):
		return []string{"zhipuai"}
	case strings.Contains(id, "kimi"):
		return []string{"moonshotai"}
	default:
		return nil
	}
}

func sameProvider(provider Provider, name, baseURL string) bool {
	name = normalized(name)
	if name != "" {
		for _, candidate := range providerNames(name) {
			if candidate == normalized(provider.ID) || candidate == normalized(provider.Name) {
				return true
			}
		}
	}
	return sameAPI(provider.API, baseURL)
}

// Some Solomon integrations are subscriptions rather than a direct API
// provider. Their model IDs still refer to the lab's canonical catalog entry.
func providerNames(name string) []string {
	names := []string{name}
	switch name {
	case "chatgptsub":
		names = append(names, "openai")
	case "claudesub":
		names = append(names, "anthropic")
	}
	return names
}

func normalized(value string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func apiHost(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

func sameAPI(left, right string) bool {
	leftURL, err := url.Parse(strings.TrimSpace(left))
	if err != nil {
		return false
	}
	rightURL, err := url.Parse(strings.TrimSpace(right))
	if err != nil || apiHost(left) == "" || apiHost(left) != apiHost(right) {
		return false
	}
	leftPath := strings.Trim(strings.ToLower(leftURL.Path), "/")
	rightPath := strings.Trim(strings.ToLower(rightURL.Path), "/")
	return leftPath == "" || rightPath == "" || leftPath == rightPath || strings.HasPrefix(rightPath, leftPath+"/")
}
