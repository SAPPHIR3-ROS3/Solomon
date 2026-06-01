package codex

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

var codexUnsupportedModelRE = regexp.MustCompile(`The '([^']+)' model is not supported when using Codex with a ChatGPT account`)

type upstreamErrorPayload struct {
	Detail  string `json:"detail"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

func chatGPTSubUpstreamError(statusCode int, body []byte, model string) error {
	msg := humanizeCodexUpstreamError(statusCode, parseCodexUpstreamDetail(body), strings.TrimSpace(model))
	return fmt.Errorf("ChatGPT Sub: %s", msg)
}

func parseCodexUpstreamDetail(body []byte) string {
	raw := strings.TrimSpace(string(body))
	if raw == "" {
		return ""
	}
	var p upstreamErrorPayload
	if json.Unmarshal(body, &p) == nil {
		if s := strings.TrimSpace(p.Detail); s != "" {
			return s
		}
		if s := strings.TrimSpace(p.Message); s != "" {
			return s
		}
	}
	return raw
}

func humanizeCodexUpstreamError(statusCode int, detail, model string) string {
	if m := humanizeKnownCodexDetail(detail, model); m != "" {
		return m
	}
	if detail != "" {
		return detail
	}
	switch statusCode {
	case http.StatusUnauthorized:
		return "sign-in expired or invalid; run /connect to sign in again"
	case http.StatusForbidden:
		return "this ChatGPT account cannot use Codex; check your subscription on chatgpt.com"
	case http.StatusTooManyRequests:
		return "usage limit reached; wait for the limit to reset or change model"
	case http.StatusBadRequest:
		return "request rejected by ChatGPT Codex; try another model with /models"
	default:
		return fmt.Sprintf("Codex API error (HTTP %d)", statusCode)
	}
}

func humanizeKnownCodexDetail(detail, model string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return ""
	}
	if m := codexUnsupportedModelRE.FindStringSubmatch(detail); len(m) == 2 {
		name := strings.TrimSpace(m[1])
		if name == "" {
			name = strings.TrimSpace(model)
		}
		if name != "" {
			return fmt.Sprintf("model %q is not available on your ChatGPT plan; use /models to pick another (free plan: gpt-5.4-mini)", name)
		}
		return "this model is not available on your ChatGPT plan; use /models to pick another (free plan: gpt-5.4-mini)"
	}
	lower := strings.ToLower(detail)
	if strings.Contains(lower, "usage limit") {
		return detail
	}
	return ""
}

func drainUpstreamError(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, nil
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
