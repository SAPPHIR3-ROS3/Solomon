package cursor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkhttp"
	"io"
	"net/http"
	"strings"
	"time"
)

const mintURL = "https://api2.cursor.sh/aiserver.v1.DashboardService/CreateUserApiKey"

var mintEndpoint = mintURL

func MintUserAPIKey(ctx context.Context, accessToken string) (string, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return "", fmt.Errorf("cursor login: missing session token")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	payload, err := json.Marshal(map[string]string{"name": "Solomon"})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mintEndpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	client := &http.Client{Transport: networkhttp.NewDefaultTransport(), Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("cursor login: mint api key: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("cursor login: mint api key status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		APIKey    string `json:"apiKey"`
		APIKeyAlt string `json:"api_key"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("cursor login: parse api key: %w", err)
	}
	key := firstNonEmpty(out.APIKey, out.APIKeyAlt)
	if key == "" {
		return "", fmt.Errorf("cursor login: mint api key missing")
	}
	return key, nil
}

const modelsURL = "https://api2.cursor.sh/aiserver.v1.AiService/AvailableModels"

var modelsEndpoint = modelsURL

type ModelFlag struct {
	ID             string
	Raw            string
	Fast           bool
	Thinking       bool
	ThinkingLevels bool
}

func ListAvailableModelFlags(ctx context.Context, accessToken string) ([]ModelFlag, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return nil, fmt.Errorf("cursor models: missing session token")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, modelsEndpoint, strings.NewReader(`{"useModelParameters":true}`))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Transport: networkhttp.NewDefaultTransport(), Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cursor models: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cursor models status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	flags := parseAvailableModelFlags(body)
	if len(flags) == 0 {
		return nil, fmt.Errorf("cursor models: empty list")
	}
	SetModelCaps(DedupeVariantFlags(flags))
	return flags, nil
}

func ListAvailableModels(ctx context.Context, accessToken string) ([]string, error) {
	flags, err := ListAvailableModelFlags(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(flags))
	for _, flag := range flags {
		ids = append(ids, flag.ID)
	}
	return ids, nil
}

func parseAvailableModelFlags(body []byte) []ModelFlag {
	var payload map[string]any
	if json.Unmarshal(body, &payload) != nil {
		return nil
	}
	raw := payload["models"]
	if raw == nil {
		raw = payload["availableModels"]
	}
	list, _ := raw.([]any)
	seen := map[string]bool{}
	out := make([]ModelFlag, 0, len(list))
	for _, item := range list {
		m, _ := item.(map[string]any)
		name := ""
		if m != nil {
			for _, key := range []string{"name", "serverModelName", "modelName", "model"} {
				text, _ := m[key].(string)
				text = strings.TrimSpace(text)
				if text != "" && !strings.Contains(text, " ") {
					name = text
					break
				}
			}
		} else if text, ok := item.(string); ok {
			name = strings.TrimSpace(text)
		}
		full := name
		name = CanonicalCursorModelID(name)
		if name == "" || seen[strings.ToLower(name)] || modelItemUnavailable(m) {
			continue
		}
		seen[strings.ToLower(name)] = true
		out = append(out, ModelFlag{
			ID:             name,
			Raw:            full,
			Fast:           strings.Contains(strings.ToLower(full), "fast") || modelTreeHasKeyword(item, "fast") || modelOffersFast(item),
			Thinking:       thinkingHas(full) || modelTreeHasKeyword(item, "thinking"),
			ThinkingLevels: thinkingHasLevels(full) || modelTreeHasKeyword(item, "thinking-low", "thinking-medium", "thinking-high"),
		})
		for _, slug := range cursorLegacySlugs(m) {
			slug = strings.TrimSpace(slug)
			canon := CanonicalCursorModelID(slug)
			if canon == "" || seen[strings.ToLower(canon)] {
				continue
			}
			seen[strings.ToLower(canon)] = true
			fast, thinking, levels := cursorVariantBits(canon)
			out = append(out, ModelFlag{ID: canon, Raw: slug, Fast: fast, Thinking: thinking, ThinkingLevels: levels})
		}
	}
	return out
}

func cursorLegacySlugs(model map[string]any) []string {
	if model == nil {
		return nil
	}
	var slugs []string
	if raw, ok := model["legacySlugs"].([]any); ok {
		for _, item := range raw {
			if text, ok := item.(string); ok {
				slugs = append(slugs, text)
			}
		}
	}
	if raw, ok := model["variants"].([]any); ok {
		for _, item := range raw {
			entry, _ := item.(map[string]any)
			if text, _ := entry["legacySlug"].(string); text != "" {
				slugs = append(slugs, text)
			}
		}
	}
	return slugs
}

func CanonicalCursorModelID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	low := strings.ToLower(id)
	if low == "auto" || low == "default" || strings.HasPrefix(low, "cursor-") {
		return id
	}
	if strings.HasPrefix(low, "grok") {
		return "cursor-" + id
	}
	return id
}

func stripCursorModelPrefix(id string) string {
	id = strings.TrimSpace(id)
	if len(id) >= 7 && strings.EqualFold(id[:7], "cursor-") {
		return strings.TrimSpace(id[7:])
	}
	return id
}

func modelItemUnavailable(m map[string]any) bool {
	if m == nil {
		return false
	}
	for _, key := range []string{"available", "isAvailable", "isUsable"} {
		if v, ok := m[key].(bool); ok && !v {
			return true
		}
	}
	for _, key := range []string{"disabled", "deprecated", "isDeprecated", "hidden", "unavailable"} {
		if v, ok := m[key].(bool); ok && v {
			return true
		}
	}
	for _, key := range []string{"status", "availability", "state"} {
		text, _ := m[key].(string)
		switch strings.ToLower(strings.TrimSpace(text)) {
		case "unavailable", "disabled", "deprecated", "hidden", "retired":
			return true
		}
	}
	return false
}

func thinkingHas(text string) bool {
	return strings.Contains(strings.ToLower(text), "thinking")
}

func thinkingHasLevels(text string) bool {
	low := strings.ToLower(text)
	return strings.Contains(low, "thinking-low") || strings.Contains(low, "thinking-medium") || strings.Contains(low, "thinking-high")
}

func modelTreeHasKeyword(value any, words ...string) bool {
	switch typed := value.(type) {
	case string:
		low := strings.ToLower(typed)
		for _, word := range words {
			if strings.Contains(low, word) {
				return true
			}
		}
	case []any:
		for _, item := range typed {
			if modelTreeHasKeyword(item, words...) {
				return true
			}
		}
	case map[string]any:
		for key, child := range typed {
			low := strings.ToLower(key)
			if strings.Contains(low, "variant") || low == "name" || low == "id" || low == "value" || low == "displayname" {
				if modelTreeHasKeyword(child, words...) {
					return true
				}
			}
		}
	}
	return false
}

func SetModelsEndpointForTest(url string) func() {
	old := modelsEndpoint
	modelsEndpoint = url
	return func() { modelsEndpoint = old }
}

func SetEndpointsForTest(poll, refresh, mint string) func() {
	oldPoll, oldRefresh, oldMint, oldDelay := pollEndpoint, refreshEndpoint, mintEndpoint, pollBaseDelay
	pollEndpoint, refreshEndpoint, mintEndpoint, pollBaseDelay = poll, refresh, mint, 0
	return func() {
		pollEndpoint, refreshEndpoint, mintEndpoint, pollBaseDelay = oldPoll, oldRefresh, oldMint, oldDelay
	}
}

func DefaultModelIDs() []string {
	return []string{"composer-2.5", "auto"}
}
