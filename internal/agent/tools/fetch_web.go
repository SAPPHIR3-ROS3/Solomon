package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"

	"github.com/openai/openai-go/v2"
)

func signatureFetchWeb(targetURL string) {}

const (
	fetchWebMaxBodyBytes     = 5 * 1024 * 1024
	fetchWebDefaultTimeoutS  = 30
	fetchWebMaxTimeoutSecs   = 120
	fetchWebUserAgent        = "Solomon-fetchWeb/1.0"
	fetchMarkdownDescription = `Download a URL via HTTP GET and return the body as Markdown. HTML pages are converted to CommonMark-style Markdown (headings, links, lists, code). Other common text types (plain, JSON, XML) are returned as fenced code blocks. Maximum response body is 5MB. Only http(s) URLs. Optional timeoutSeconds (default 30, max 120).`
)

type fetchWebArgs struct {
	URL            string `json:"url"`
	TimeoutSeconds *int   `json:"timeoutSeconds,omitempty"`
}

func fetchWebOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("fetchWeb", fetchMarkdownDescription, map[string]any{
		"url": map[string]any{"type": "string", "description": "Fully qualified http or https URL to fetch"},
		"timeoutSeconds": map[string]any{
			"type":        "integer",
			"description": fmt.Sprintf("Optional timeout in seconds (default %d, max %d)", fetchWebDefaultTimeoutS, fetchWebMaxTimeoutSecs),
			"minimum":     1,
			"maximum":     fetchWebMaxTimeoutSecs,
		},
	}, []string{"url"})
}

func appendFetchWebDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureFetchWeb)
	if err != nil {
		return err
	}
	b.addBlock("fetchWeb", fetchMarkdownDescription, sig)
	return nil
}

func execFetchWeb(ctx context.Context, raw json.RawMessage) (any, error) {
	var a fetchWebArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	a.URL = strings.TrimSpace(a.URL)
	if a.URL == "" {
		return nil, fmt.Errorf("fetchWeb: empty url")
	}
	u, err := url.Parse(a.URL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("fetchWeb: invalid url")
	}
	if s := strings.ToLower(u.Scheme); s != "http" && s != "https" {
		return nil, fmt.Errorf("fetchWeb: only http and https URLs are allowed")
	}
	sec := fetchWebDefaultTimeoutS
	if a.TimeoutSeconds != nil && *a.TimeoutSeconds > 0 {
		sec = *a.TimeoutSeconds
		if sec > fetchWebMaxTimeoutSecs {
			sec = fetchWebMaxTimeoutSecs
		}
	}
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(sec)*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, a.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("fetchWeb: build request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/markdown,text/plain;q=0.9,application/json;q=0.8,*/*;q=0.1")
	req.Header.Set("User-Agent", fetchWebUserAgent)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetchWeb: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("fetchWeb: HTTP %d", resp.StatusCode)
	}
	ct := mimeFromContentType(resp.Header.Get("Content-Type"))
	body, err := io.ReadAll(io.LimitReader(resp.Body, fetchWebMaxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("fetchWeb: read body: %w", err)
	}
	if len(body) > fetchWebMaxBodyBytes {
		return nil, fmt.Errorf("fetchWeb: response body exceeds %d bytes", fetchWebMaxBodyBytes)
	}
	rawStr := string(body)
	md, err := bytesToMarkdown(rawStr, ct)
	if err != nil {
		return nil, err
	}
	finalURL := a.URL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return map[string]any{
		"url":         finalURL,
		"status":      resp.StatusCode,
		"contentType": strings.TrimSpace(resp.Header.Get("Content-Type")),
		"markdown":    md,
	}, nil
}

func mimeFromContentType(h string) string {
	h = strings.TrimSpace(strings.Split(h, ";")[0])
	return strings.ToLower(h)
}

func bytesToMarkdown(body string, mime string) (string, error) {
	switch {
	case strings.Contains(mime, "html"):
		out, err := htmltomarkdown.ConvertString(body)
		if err != nil {
			return "", fmt.Errorf("fetchWeb: html to markdown: %w", err)
		}
		return strings.TrimSpace(out), nil
	case mime == "" && looksLikeHTMLSnippet(body):
		out, err := htmltomarkdown.ConvertString(body)
		if err != nil {
			return "", fmt.Errorf("fetchWeb: html to markdown: %w", err)
		}
		return strings.TrimSpace(out), nil
	case mime == "":
		return fenceMarkdown("", strings.TrimRight(body, "\r\n")), nil
	case strings.Contains(mime, "markdown"):
		return strings.TrimSpace(body), nil
	case strings.Contains(mime, "json"):
		return fenceMarkdown("json", body), nil
	case strings.Contains(mime, "xml") || mime == "text/xml" || mime == "application/xml":
		return fenceMarkdown("xml", body), nil
	case strings.HasPrefix(mime, "text/"):
		return fenceMarkdown("", strings.TrimRight(body, "\r\n")), nil
	default:
		if looksLikeHTMLSnippet(body) {
			out, err := htmltomarkdown.ConvertString(body)
			if err == nil && strings.TrimSpace(out) != "" {
				return strings.TrimSpace(out), nil
			}
		}
		return "", fmt.Errorf("fetchWeb: unsupported content-type %q (not text/html-like)", mime)
	}
}

func looksLikeHTMLSnippet(s string) bool {
	t := strings.TrimLeft(s, " \n\r\t")
	if len(t) == 0 {
		return false
	}
	head := strings.ToLower(t)
	if len(head) > 1024 {
		head = head[:1024]
	}
	return strings.HasPrefix(head, "<!doctype") || strings.Contains(head, "<html") ||
		strings.Contains(head, "<head") || strings.Contains(head, "<body")
}

func fenceMarkdown(lang, body string) string {
	body = strings.TrimRight(body, "\r\n") + "\n"
	if lang == "" {
		return "```\n" + body + "```"
	}
	return "```" + lang + "\n" + body + "```"
}
