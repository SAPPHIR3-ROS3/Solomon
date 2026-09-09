package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// MCPToolCaller is the narrow host-side surface needed by MCP-backed search
// adapters. The concrete MCP manager remains outside this package so the
// adapter can be unit-tested without a live remote server.
type MCPToolCaller interface {
	CallInternalTool(ctx context.Context, serverName, toolName, intent string, args map[string]any) (any, error)
}

var markdownResultLink = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^)\s]+)\)`)

var browserSnapshotLink = regexp.MustCompile(`(?i)\blink\s+"([^"]+)"`)

var browserSnapshotURL = regexp.MustCompile(`(?i)^\s*-?\s*/url:\s*(https?://\S+)`)

func normalizeMCPResponse(raw any, provider string, maxResults int) (Response, error) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	if provider == "" {
		provider = "mcp"
	}
	if maxResults <= 0 {
		maxResults = 10
	}

	if message := mcpErrorMessage(raw); message != "" {
		return Response{}, fmt.Errorf("%s: MCP search failed: %s", provider, message)
	}

	metadata := &ResponseMetadata{Provider: provider, Adapter: provider}
	hits := make([]Hit, 0, maxResults)
	walkMCPValue(raw, &hits, metadata)
	if len(hits) == 0 {
		return Response{}, fmt.Errorf("%s: %w", provider, ErrNoResults)
	}

	response := Response{Hits: hits, Metadata: metadata}
	if len(response.Hits) > maxResults {
		response.HasMore = true
		response.Hits = response.Hits[:maxResults]
	}
	return response, nil
}

func walkMCPValue(value any, hits *[]Hit, metadata *ResponseMetadata) {
	switch v := value.(type) {
	case nil:
		return
	case json.RawMessage:
		var decoded any
		if err := json.Unmarshal(v, &decoded); err == nil {
			walkMCPValue(decoded, hits, metadata)
		}
	case []byte:
		var decoded any
		if err := json.Unmarshal(v, &decoded); err == nil {
			walkMCPValue(decoded, hits, metadata)
		} else {
			walkMCPText(string(v), hits)
		}
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return
		}
		var decoded any
		if err := json.Unmarshal([]byte(text), &decoded); err == nil {
			walkMCPValue(decoded, hits, metadata)
			return
		}
		walkMCPText(text, hits)
	case []any:
		for _, item := range v {
			walkMCPValue(item, hits, metadata)
		}
	case map[string]any:
		updateResponseMetadata(v, metadata)
		if hit, ok := hitFromMap(v); ok {
			appendHit(hits, hit)
			return
		}
		for _, item := range v {
			walkMCPValue(item, hits, metadata)
		}
	}
}

func walkMCPText(text string, hits *[]Hit) {
	for _, hit := range parseTextHits(text) {
		appendHit(hits, hit)
	}
}

func parseTextHits(text string) []Hit {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	var labeled []Hit
	for _, block := range strings.Split(text, "\n---\n") {
		if hit, ok := parseLabeledHit(block); ok {
			labeled = append(labeled, hit)
		}
	}
	if len(labeled) > 0 {
		return labeled
	}
	if browserHits := parseBrowserSnapshotHits(text); len(browserHits) > 0 {
		return browserHits
	}

	matches := markdownResultLink.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]Hit, 0, len(matches))
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		out = append(out, Hit{Title: strings.TrimSpace(match[1]), URL: strings.TrimSpace(match[2])})
	}
	return out
}

func parseBrowserSnapshotHits(text string) []Hit {
	var out []Hit
	pendingTitle := ""
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if match := browserSnapshotLink.FindStringSubmatch(line); len(match) > 1 {
			pendingTitle = strings.TrimSpace(match[1])
		}
		match := browserSnapshotURL.FindStringSubmatch(line)
		if len(match) < 2 {
			continue
		}
		link := strings.TrimRight(strings.TrimSpace(match[1]), ".,;:!?)]}\"")
		out = append(out, Hit{Title: pendingTitle, URL: link})
		pendingTitle = ""
	}
	return out
}

func parseLabeledHit(block string) (Hit, bool) {
	var hit Hit
	var body []string
	mode := ""
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Title:"):
			hit.Title = strings.TrimSpace(strings.TrimPrefix(line, "Title:"))
		case strings.HasPrefix(line, "URL:"):
			hit.URL = strings.TrimSpace(strings.TrimPrefix(line, "URL:"))
		case strings.HasPrefix(line, "Published:"):
			hit.PublishedAt = strings.TrimSpace(strings.TrimPrefix(line, "Published:"))
		case strings.HasPrefix(line, "Author:"):
			hit.Author = strings.TrimSpace(strings.TrimPrefix(line, "Author:"))
		case strings.HasPrefix(line, "Highlights:"):
			mode = "snippet"
			if suffix := strings.TrimSpace(strings.TrimPrefix(line, "Highlights:")); suffix != "" {
				body = append(body, suffix)
			}
		case strings.HasPrefix(line, "Text:"):
			mode = "content"
			if suffix := strings.TrimSpace(strings.TrimPrefix(line, "Text:")); suffix != "" {
				body = append(body, suffix)
			}
		default:
			if mode != "" && line != "" {
				body = append(body, line)
			}
		}
	}
	if hit.URL == "" {
		return Hit{}, false
	}
	if len(body) > 0 {
		if mode == "content" {
			hit.Content = strings.Join(body, "\n")
			hit.Snippet = hit.Content
		} else {
			hit.Snippet = strings.Join(body, "\n")
		}
	}
	return hit, true
}

func hitFromMap(value map[string]any) (Hit, bool) {
	link := firstString(value, "url", "URL", "link", "source_url", "sourceUrl")
	if link == "" {
		return Hit{}, false
	}
	hit := Hit{
		Title:       firstString(value, "title", "name"),
		URL:         link,
		Author:      firstString(value, "author"),
		PublishedAt: firstString(value, "publishedDate", "published_date", "publish_date", "publishedAt"),
	}
	hit.Snippet = firstString(value, "snippet", "description", "summary")
	if excerpts := stringSlice(value, "highlights", "excerpts"); len(excerpts) > 0 {
		hit.Snippet = strings.Join(excerpts, "\n\n")
	}
	hit.Content = firstString(value, "text", "full_content", "fullContent")
	if hit.Snippet == "" && hit.Content != "" {
		hit.Snippet = hit.Content
	}
	if score, ok := numberField(value, "score", "relevance_score", "relevanceScore"); ok {
		hit.Score = &score
	}
	if extra, ok := value["metadata"].(map[string]any); ok && len(extra) > 0 {
		hit.Metadata = cloneMap(extra)
	}
	if hit.Title == "" {
		hit.Title = hit.URL
	}
	return hit, true
}

func appendHit(hits *[]Hit, hit Hit) {
	link, ok := normalizeHitURL(hit.URL)
	if !ok {
		return
	}
	hit.URL = link
	if hit.Title == "" {
		hit.Title = link
	}
	for i := range *hits {
		if (*hits)[i].URL != hit.URL {
			continue
		}
		mergeHit(&(*hits)[i], hit)
		return
	}
	*hits = append(*hits, hit)
}

func mergeHit(dst *Hit, src Hit) {
	if dst.Title == "" {
		dst.Title = src.Title
	}
	if dst.Snippet == "" {
		dst.Snippet = src.Snippet
	}
	if dst.Content == "" {
		dst.Content = src.Content
	}
	if dst.Author == "" {
		dst.Author = src.Author
	}
	if dst.PublishedAt == "" {
		dst.PublishedAt = src.PublishedAt
	}
	if dst.Score == nil {
		dst.Score = src.Score
	}
	if len(src.Metadata) > 0 {
		if dst.Metadata == nil {
			dst.Metadata = map[string]any{}
		}
		for key, value := range src.Metadata {
			if _, exists := dst.Metadata[key]; !exists {
				dst.Metadata[key] = value
			}
		}
	}
}

func normalizeHitURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", false
	}
	if scheme := strings.ToLower(u.Scheme); scheme != "http" && scheme != "https" {
		return "", false
	}
	return u.String(), true
}

func mcpErrorMessage(value any) string {
	if object, ok := value.(map[string]any); ok {
		if message := firstString(object, "error", "errorMessage"); message != "" {
			return message
		}
		if nested, ok := object["structuredContent"]; ok {
			if message := mcpErrorMessage(nested); message != "" {
				return message
			}
		}
	}
	return ""
}

func updateResponseMetadata(value map[string]any, metadata *ResponseMetadata) {
	if metadata == nil {
		return
	}
	if metadata.SessionID == "" {
		metadata.SessionID = firstString(value, "session_id", "sessionId")
	}
	if metadata.ProviderRequestID == "" {
		metadata.ProviderRequestID = firstString(value, "search_id", "searchId", "request_id", "requestId")
	}
	if warnings := stringSlice(value, "warnings"); len(warnings) > 0 {
		metadata.Warnings = appendUniqueStrings(metadata.Warnings, warnings...)
	}
	if usage, ok := value["usage"]; ok {
		if metadata.ProviderData == nil {
			metadata.ProviderData = map[string]any{}
		}
		metadata.ProviderData["usage"] = usage
	}
}

func firstString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := value[key]; ok {
			if text, ok := raw.(string); ok && strings.TrimSpace(text) != "" {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func stringSlice(value map[string]any, keys ...string) []string {
	for _, key := range keys {
		raw, ok := value[key]
		if !ok || raw == nil {
			continue
		}
		switch items := raw.(type) {
		case []string:
			out := make([]string, 0, len(items))
			for _, item := range items {
				if item = strings.TrimSpace(item); item != "" {
					out = append(out, item)
				}
			}
			if len(out) > 0 {
				return out
			}
		case []any:
			out := make([]string, 0, len(items))
			for _, item := range items {
				if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
					out = append(out, strings.TrimSpace(text))
				}
			}
			if len(out) > 0 {
				return out
			}
		case string:
			if text := strings.TrimSpace(items); text != "" {
				return []string{text}
			}
		}
	}
	return nil
}

func numberField(value map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		raw, ok := value[key]
		if !ok || raw == nil {
			continue
		}
		switch number := raw.(type) {
		case float64:
			return number, true
		case float32:
			return float64(number), true
		case int:
			return float64(number), true
		case int64:
			return float64(number), true
		case json.Number:
			parsed, err := number.Float64()
			if err == nil {
				return parsed, true
			}
		case string:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(number), 64)
			if err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func cloneMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	out := make(map[string]any, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}

func appendUniqueStrings(dst []string, values ...string) []string {
	seen := make(map[string]struct{}, len(dst)+len(values))
	for _, item := range dst {
		seen[item] = struct{}{}
	}
	for _, item := range values {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		dst = append(dst, item)
	}
	return dst
}
