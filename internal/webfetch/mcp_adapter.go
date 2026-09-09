package webfetch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// MCPToolCaller is the narrow host-side surface needed by MCP-backed fetch
// adapters. The concrete MCP manager stays outside this package so adapters
// remain unit-testable without a live remote server.
type MCPToolCaller interface {
	CallInternalTool(ctx context.Context, serverName, toolName, intent string, args map[string]any) (any, error)
}

type fetchDocument struct {
	URL         string
	Title       string
	Status      int
	ContentType string
	Content     []string
	Errors      []string
}

func normalizeMCPFetchResponse(raw any, provider, requestedURL string) (Result, error) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	if provider == "" {
		provider = "mcp"
	}
	requestedURL = strings.TrimSpace(requestedURL)
	doc := &fetchDocument{URL: requestedURL}
	walkMCPFetchValue(raw, doc)
	if len(doc.Content) == 0 {
		if len(doc.Errors) > 0 {
			return Result{}, fmt.Errorf("%s: %w: %s", provider, ErrNoContent, strings.Join(doc.Errors, "; "))
		}
		return Result{}, fmt.Errorf("%s: %w", provider, ErrNoContent)
	}
	content := strings.TrimSpace(strings.Join(doc.Content, "\n\n"))
	if content == "" {
		return Result{}, fmt.Errorf("%s: %w", provider, ErrNoContent)
	}
	if doc.URL == "" {
		doc.URL = requestedURL
	}
	if doc.Title == "" {
		doc.Title = doc.URL
	}
	if doc.Status == 0 {
		doc.Status = httpOK
	}
	if doc.ContentType == "" {
		doc.ContentType = "text/markdown"
	}
	metadata := &Metadata{Provider: provider, Adapter: provider}
	if len(doc.Errors) > 0 {
		metadata.Partial = true
		metadata.ProviderData = map[string]any{"errors": append([]string(nil), doc.Errors...)}
	}
	return Result{
		URL:         doc.URL,
		Status:      doc.Status,
		ContentType: doc.ContentType,
		Markdown:    content,
		Title:       doc.Title,
		Metadata:    metadata,
	}, nil
}

// Avoid importing net/http just for the conventional successful status code.
const httpOK = 200

func walkMCPFetchValue(value any, doc *fetchDocument) {
	switch v := value.(type) {
	case nil:
		return
	case json.RawMessage:
		var decoded any
		if err := json.Unmarshal(v, &decoded); err == nil {
			walkMCPFetchValue(decoded, doc)
		}
	case []byte:
		var decoded any
		if err := json.Unmarshal(v, &decoded); err == nil {
			walkMCPFetchValue(decoded, doc)
		} else {
			walkMCPFetchText(string(v), doc)
		}
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return
		}
		var decoded any
		if err := json.Unmarshal([]byte(text), &decoded); err == nil {
			walkMCPFetchValue(decoded, doc)
			return
		}
		walkMCPFetchText(text, doc)
	case []any:
		for _, item := range v {
			walkMCPFetchValue(item, doc)
		}
	case map[string]any:
		if message := fetchErrorField(v); message != "" {
			doc.Errors = appendUniqueString(doc.Errors, message)
		}
		if record, ok := fetchRecordFromMap(v); ok {
			mergeFetchRecord(doc, record)
			return
		}
		for _, key := range []string{"structuredContent", "results", "result", "content", "data", "text", "markdown", "full_content", "fullContent", "body"} {
			if child, ok := v[key]; ok {
				walkMCPFetchValue(child, doc)
			}
		}
	}
}

func walkMCPFetchText(text string, doc *fetchDocument) {
	if isFetchFailureText(text) {
		doc.Errors = appendUniqueString(doc.Errors, text)
		return
	}
	if record, ok := parseFetchText(text); ok {
		mergeFetchRecord(doc, record)
		return
	}
	appendFetchContent(doc, text)
}

type parsedFetchRecord struct {
	URL         string
	Title       string
	Status      int
	ContentType string
	Content     string
}

func fetchRecordFromMap(value map[string]any) (parsedFetchRecord, bool) {
	content, ok := firstFetchContent(value)
	if !ok {
		return parsedFetchRecord{}, false
	}
	return parsedFetchRecord{
		URL:         firstString(value, "url", "URL", "source_url", "sourceUrl"),
		Title:       firstString(value, "title", "name"),
		Status:      firstInt(value, "status", "status_code", "statusCode"),
		ContentType: firstString(value, "content_type", "contentType", "mime_type", "mimeType"),
		Content:     content,
	}, true
}

func firstFetchContent(value map[string]any) (string, bool) {
	for _, key := range []string{"markdown", "full_content", "fullContent", "text", "body"} {
		if text := fetchTextValue(value[key]); strings.TrimSpace(text) != "" {
			return text, true
		}
	}
	if raw, ok := value["content"]; ok {
		if text := fetchTextValue(raw); strings.TrimSpace(text) != "" {
			return text, true
		}
	}
	if raw, ok := value["excerpts"]; ok {
		if text := fetchTextValue(raw); strings.TrimSpace(text) != "" {
			return text, true
		}
	}
	return "", false
}

func fetchTextValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []string:
		return strings.TrimSpace(strings.Join(v, "\n\n"))
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if text := fetchTextValue(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n\n"))
	case map[string]any:
		if text, ok := firstFetchContent(v); ok {
			return text
		}
		return firstString(v, "text", "value")
	default:
		return ""
	}
}

func parseFetchText(text string) (parsedFetchRecord, bool) {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	var record parsedFetchRecord
	contentStart := -1
	seenURL := false
	for index, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		switch {
		case strings.HasPrefix(line, "# ") && record.Title == "":
			record.Title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
		case strings.HasPrefix(line, "Title:") && record.Title == "":
			record.Title = strings.TrimSpace(strings.TrimPrefix(line, "Title:"))
		case strings.HasPrefix(line, "URL:"):
			record.URL = strings.TrimSpace(strings.TrimPrefix(line, "URL:"))
			seenURL = record.URL != ""
		case strings.HasPrefix(line, "Status:"):
			record.Status, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Status:")))
		case strings.HasPrefix(line, "Content-Type:"):
			record.ContentType = strings.TrimSpace(strings.TrimPrefix(line, "Content-Type:"))
		case seenURL && contentStart < 0 && line == "":
			contentStart = index + 1
		}
	}
	if !seenURL {
		return parsedFetchRecord{}, false
	}
	if contentStart < 0 {
		contentStart = len(lines)
		for index, rawLine := range lines {
			line := strings.TrimSpace(rawLine)
			if strings.HasPrefix(line, "Author:") || strings.HasPrefix(line, "Published:") {
				continue
			}
			if index > 0 && line != "" && !strings.HasPrefix(line, "# ") && !strings.HasPrefix(line, "Title:") && !strings.HasPrefix(line, "URL:") {
				contentStart = index
				break
			}
		}
	}
	if contentStart < len(lines) {
		record.Content = strings.TrimSpace(strings.Join(lines[contentStart:], "\n"))
	}
	return record, strings.TrimSpace(record.Content) != ""
}

func mergeFetchRecord(doc *fetchDocument, record parsedFetchRecord) {
	if record.URL != "" {
		if parsed, err := url.Parse(record.URL); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			doc.URL = record.URL
		}
	}
	if record.Title != "" {
		doc.Title = record.Title
	}
	if record.Status != 0 {
		doc.Status = record.Status
	}
	if record.ContentType != "" {
		doc.ContentType = record.ContentType
	}
	appendFetchContent(doc, record.Content)
}

func appendFetchContent(doc *fetchDocument, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	for _, existing := range doc.Content {
		if existing == content {
			return
		}
	}
	doc.Content = append(doc.Content, content)
}

func fetchErrorField(value map[string]any) string {
	if raw, ok := value["error"]; ok {
		if message := fetchTextValue(raw); message != "" {
			return message
		}
	}
	if isError, ok := value["isError"].(bool); ok && isError {
		if message := fetchTextValue(value["message"]); message != "" {
			return message
		}
		return "MCP tool returned an error"
	}
	return ""
}

func isFetchFailureText(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return lower == "no content found." ||
		lower == "no content found for the provided url(s)." ||
		strings.HasPrefix(lower, "error fetching url")
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func firstString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if text, ok := value[key].(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func firstInt(value map[string]any, keys ...string) int {
	for _, key := range keys {
		switch number := value[key].(type) {
		case int:
			return number
		case int64:
			return int(number)
		case float64:
			return int(number)
		case string:
			parsed, _ := strconv.Atoi(strings.TrimSpace(number))
			if parsed != 0 {
				return parsed
			}
		}
	}
	return 0
}

func extraString(extras map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := extras[key]; ok {
			switch text := value.(type) {
			case string:
				if strings.TrimSpace(text) != "" {
					return strings.TrimSpace(text)
				}
			}
		}
	}
	return ""
}

func extraStringSlice(extras map[string]any, keys ...string) []string {
	for _, key := range keys {
		value, ok := extras[key]
		if !ok {
			continue
		}
		switch list := value.(type) {
		case []string:
			return append([]string(nil), list...)
		case []any:
			out := make([]string, 0, len(list))
			for _, item := range list {
				if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
					out = append(out, strings.TrimSpace(text))
				}
			}
			return out
		}
	}
	return nil
}

func extraInt(extras map[string]any, keys ...string) int {
	for _, key := range keys {
		value, ok := extras[key]
		if !ok {
			continue
		}
		switch number := value.(type) {
		case int:
			return number
		case int64:
			return int(number)
		case float64:
			return int(number)
		case string:
			parsed, err := strconv.Atoi(strings.TrimSpace(number))
			if err == nil {
				return parsed
			}
		}
	}
	return 0
}
