package chat

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var codexUnsupportedModelRE = regexp.MustCompile(`The '([^']+)' model is not supported when using Codex with a ChatGPT account`)

type upstreamErrorPayload struct {
	Detail          string                `json:"detail"`
	Message         string                `json:"message"`
	Type            string                `json:"type"`
	PlanType        string                `json:"plan_type"`
	ResetsAt        int64                 `json:"resets_at"`
	ResetsInSeconds int64                 `json:"resets_in_seconds"`
	Error           *upstreamErrorPayload `json:"error"`
}

func ChatGPTSubUpstreamError(statusCode int, body []byte, model string) error {
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
		if s := formatCodexPayload(p); s != "" {
			return s
		}
	}
	return raw
}

func formatCodexPayload(p upstreamErrorPayload) string {
	if p.Error != nil {
		if s := formatCodexPayload(*p.Error); s != "" {
			return s
		}
	}
	if s := formatUsageLimitDetail(p); s != "" {
		return s
	}
	if s := strings.TrimSpace(p.Detail); s != "" {
		return s
	}
	return strings.TrimSpace(p.Message)
}

func formatUsageLimitDetail(p upstreamErrorPayload) string {
	typ := strings.TrimSpace(p.Type)
	msg := strings.TrimSpace(p.Message)
	if typ != "usage_limit_reached" && typ != "rate_limit_error" && !strings.Contains(strings.ToLower(msg), "usage limit") {
		return ""
	}
	if msg == "" {
		msg = "The usage limit has been reached"
	}
	if plan := strings.TrimSpace(p.PlanType); plan != "" {
		msg += " (" + plan + " plan)"
	}
	if reset := formatUsageReset(p.ResetsAt, p.ResetsInSeconds); reset != "" {
		msg += "; resets " + reset
	}
	return msg
}

func formatUsageReset(resetsAt, resetsInSeconds int64) string {
	var when string
	if resetsAt > 0 {
		when = time.Unix(resetsAt, 0).Local().Format("2006-01-02 15:04")
	}
	sec := resetsInSeconds
	if sec <= 0 && resetsAt > 0 {
		sec = int64(time.Until(time.Unix(resetsAt, 0)).Seconds())
		if sec < 0 {
			sec = 0
		}
	}
	wait := formatResetWait(sec)
	switch {
	case when != "" && wait != "":
		return when + " (" + wait + ")"
	case when != "":
		return when
	default:
		return wait
	}
}

func formatResetWait(sec int64) string {
	if sec <= 0 {
		return ""
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("in %dh %dm", h, m)
	case h > 0:
		return fmt.Sprintf("in %dh", h)
	case m > 0:
		return fmt.Sprintf("in %dm", m)
	default:
		return fmt.Sprintf("in %ds", sec)
	}
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

func DrainUpstreamError(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, nil
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
