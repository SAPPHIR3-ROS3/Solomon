package cursor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
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
	client := &http.Client{Timeout: 8 * time.Second}
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, modelsEndpoint, strings.NewReader("{}"))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
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
			Fast:           strings.Contains(strings.ToLower(full), "fast") || modelTreeHasKeyword(item, "fast"),
			Thinking:       thinkingHas(full) || modelTreeHasKeyword(item, "thinking"),
			ThinkingLevels: thinkingHasLevels(full) || modelTreeHasKeyword(item, "thinking-low", "thinking-medium", "thinking-high"),
		})
	}
	return out
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

func DedupeVariantIDs(ids []string) []string {
	flags := make([]ModelFlag, 0, len(ids))
	for _, id := range ids {
		fast, thinking, levels := cursorVariantBits(id)
		flags = append(flags, ModelFlag{ID: id, Fast: fast, Thinking: thinking, ThinkingLevels: levels})
	}
	return DedupeVariantFlags(flags).IDs
}

type ModelCaps struct {
	IDs            []string
	Fast           []string
	ThinkingToggle []string
	ThinkingLevels []string
	Variants       []string
}

func DedupeVariantFlags(items []ModelFlag) ModelCaps {
	best := map[string]ModelFlag{}
	order := make([]string, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" || isCursorVariantLabel(id) {
			continue
		}
		base := cursorBaseModelID(id)
		if base == "" {
			continue
		}
		key := strings.ToLower(base)
		fast, thinking, levels := cursorVariantBits(id)
		item.Fast = item.Fast || fast
		item.Thinking = item.Thinking || thinking
		item.ThinkingLevels = item.ThinkingLevels || levels
		prev, exists := best[key]
		if !exists {
			best[key] = ModelFlag{ID: id, Fast: item.Fast, Thinking: item.Thinking, ThinkingLevels: item.ThinkingLevels}
			order = append(order, key)
			continue
		}
		if strings.EqualFold(id, base) && !strings.EqualFold(prev.ID, base) {
			prev.ID = id
		}
		prev.Fast = prev.Fast || item.Fast
		prev.Thinking = prev.Thinking || item.Thinking
		prev.ThinkingLevels = prev.ThinkingLevels || item.ThinkingLevels
		best[key] = prev
	}
	caps := ModelCaps{}
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" || isCursorVariantLabel(id) {
			continue
		}
		raw := strings.TrimSpace(item.Raw)
		if raw == "" {
			raw = id
		}
		caps.Variants = append(caps.Variants, raw)
	}
	for _, key := range order {
		item := best[key]
		id := item.ID
		if !strings.EqualFold(id, key) {
			id = cursorBaseModelID(id)
		}
		caps.IDs = append(caps.IDs, id)
		if item.Fast {
			caps.Fast = append(caps.Fast, id)
		}
		if item.ThinkingLevels {
			caps.ThinkingLevels = append(caps.ThinkingLevels, id)
		} else if item.Thinking {
			caps.ThinkingToggle = append(caps.ThinkingToggle, id)
		}
	}
	return caps
}

var (
	fastMu         sync.Mutex
	fastLoaded     bool
	fastAllowed    = map[string]bool{}
	thinkingToggle = map[string]bool{}
	thinkingLevels = map[string]bool{}
	knownVariants  []string
)

func SetModelCaps(caps ModelCaps) {
	fastMu.Lock()
	defer fastMu.Unlock()
	fastLoaded = true
	fastAllowed = map[string]bool{}
	thinkingToggle = map[string]bool{}
	thinkingLevels = map[string]bool{}
	knownVariants = append([]string(nil), caps.Variants...)
	for _, id := range caps.Fast {
		if id = strings.ToLower(strings.TrimSpace(id)); id != "" {
			fastAllowed[id] = true
		}
	}
	for _, id := range caps.ThinkingToggle {
		if id = strings.ToLower(strings.TrimSpace(id)); id != "" {
			thinkingToggle[id] = true
		}
	}
	for _, id := range caps.ThinkingLevels {
		if id = strings.ToLower(strings.TrimSpace(id)); id != "" {
			thinkingLevels[id] = true
		}
	}
}

func FastModelIDs() []string {
	return capMapKeys(fastAllowed)
}

func ModelSupportsFast(id string) bool {
	fastMu.Lock()
	defer fastMu.Unlock()
	if !fastLoaded {
		return true
	}
	return fastAllowed[strings.ToLower(cursorBaseModelID(id))]
}

func ThinkingMode(id string) string {
	fastMu.Lock()
	defer fastMu.Unlock()
	if !fastLoaded {
		return ""
	}
	key := strings.ToLower(cursorBaseModelID(id))
	if thinkingLevels[key] {
		return "levels"
	}
	if thinkingToggle[key] {
		return "toggle"
	}
	return "none"
}

func ThinkingToggleIDs() []string {
	return capMapKeys(thinkingToggle)
}

func ThinkingLevelIDs() []string {
	return capMapKeys(thinkingLevels)
}

func capMapKeys(values map[string]bool) []string {
	fastMu.Lock()
	defer fastMu.Unlock()
	if !fastLoaded {
		return nil
	}
	out := make([]string, 0, len(values))
	for id := range values {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func isCursorVariantLabel(id string) bool {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "fast", "thinking", "max", "default", "high", "low", "medium", "mid", "xhigh", "extra-high", "auto-fast", "none":
		return true
	default:
		return false
	}
}

func cursorVariantBits(id string) (fast, thinking, levels bool) {
	low := strings.ToLower(strings.TrimSpace(id))
	fast = strings.Contains(low, "fast")
	thinking = strings.Contains(low, "thinking")
	for _, token := range []string{"thinking-low", "thinking-medium", "thinking-high", "-xhigh", "-extra-high"} {
		if strings.Contains(low, token) {
			return fast, true, true
		}
	}
	for _, suf := range []string{"-high", "-low", "-medium", "-mid"} {
		if strings.HasSuffix(low, suf) {
			return fast, true, true
		}
	}
	return fast, thinking, false
}

func cursorBaseModelID(id string) string {
	id = strings.TrimSpace(id)
	prefix := ""
	if len(id) >= 7 && strings.EqualFold(id[:7], "cursor-") {
		prefix = id[:7]
		id = id[7:]
	}
	if i := strings.IndexAny(id, "[(:"); i > 0 {
		id = strings.TrimSpace(id[:i])
	}
	for {
		n := len(id)
		low := strings.ToLower(id)
		for _, suf := range []string{
			"-thinking-medium", "-thinking-low", "-thinking-high", "-thinking",
			"-fast", "-max-mode", "-extra-high", "-xhigh",
			"-high", "-medium", "-mid", "-low", "-max", "-none",
		} {
			if strings.HasSuffix(low, suf) {
				id = strings.TrimRight(id[:len(id)-len(suf)], "-_")
				break
			}
		}
		if len(id) == n {
			return prefix + id
		}
	}
}
