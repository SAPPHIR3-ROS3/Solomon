package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/transport"
)

type codexAPIErrorBody struct {
	Type            string `json:"type"`
	Message         string `json:"message"`
	Detail          string `json:"detail"`
	PlanType        string `json:"plan_type"`
	ResetsAt        int64  `json:"resets_at"`
	ResetsInSeconds int64  `json:"resets_in_seconds"`
}

func UserFacingAPIError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrCircuitOpen) {
		return strings.Join([]string{
			"summary: provider temporarily unavailable",
			"hint: Solomon paused API calls after repeated failures. Wait about 60 seconds, then send another message.",
		}, "\n")
	}
	if msg := chatGPTSubPrefixedError(err); msg != "" {
		return msg
	}
	var phe *transport.ProviderHTTPError
	if errors.As(err, &phe) {
		return formatAPIErrorLines(0, phe.StatusCode, phe.Message)
	}
	msg := err.Error()
	attempts := 0
	if rest, n, ok := parseAfterAttemptsPrefix(msg); ok {
		attempts = n
		msg = rest
	}
	if body, requestID, ok := extractAPIErrorPayload(msg); ok {
		formatted := formatAPIErrorLines(attempts, statusCodeFromMessage(msg), body, requestID)
		if strings.HasPrefix(msg, "models API: ") {
			formatted = "source: models API\n" + formatted
		}
		return formatted
	}
	if formatted, ok := formatModelsAPIPlainError(msg); ok {
		return formatted
	}
	if formatted, ok := formatHTTPRequestError(msg); ok {
		if attempts > 0 {
			return fmt.Sprintf("attempts: %d\n%s", attempts, formatted)
		}
		return formatted
	}
	if attempts > 0 {
		return fmt.Sprintf("attempts: %d\n%s", attempts, msg)
	}
	return msg
}

func parseAfterAttemptsPrefix(msg string) (rest string, attempts int, ok bool) {
	const prefix = "after "
	if !strings.HasPrefix(msg, prefix) {
		return msg, 0, false
	}
	rest = msg[len(prefix):]
	i := strings.Index(rest, " attempt(s): ")
	if i < 0 {
		return msg, 0, false
	}
	n, err := strconv.Atoi(rest[:i])
	if err != nil || n < 1 {
		return msg, 0, false
	}
	return rest[i+len(" attempt(s): "):], n, true
}

func extractAPIErrorPayload(msg string) (codexAPIErrorBody, string, bool) {
	idx := strings.Index(msg, "{")
	if idx < 0 {
		return codexAPIErrorBody{}, "", false
	}
	raw := strings.TrimSpace(msg[idx:])
	var requestID string
	var wrapped struct {
		Error     codexAPIErrorBody `json:"error"`
		RequestID string            `json:"request_id"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapped); err == nil {
		requestID = strings.TrimSpace(wrapped.RequestID)
		body := wrapped.Error
		if body.Type != "" || body.Message != "" || body.Detail != "" {
			return body, requestID, true
		}
	}
	var body codexAPIErrorBody
	if err := json.Unmarshal([]byte(raw), &body); err == nil {
		if body.Type != "" || body.Message != "" || body.Detail != "" || body.ResetsAt != 0 || body.ResetsInSeconds != 0 {
			return body, requestID, true
		}
	}
	return codexAPIErrorBody{}, "", false
}

func chatGPTSubPrefixedError(err error) string {
	const prefix = "ChatGPT Sub: "
	msg := strings.TrimSpace(err.Error())
	if strings.HasPrefix(msg, prefix) {
		return strings.TrimSpace(msg[len(prefix):])
	}
	if rest, n, ok := parseAfterAttemptsPrefix(msg); ok && strings.HasPrefix(rest, prefix) {
		if n > 1 {
			return fmt.Sprintf("attempts: %d\n%s", n, strings.TrimSpace(rest[len(prefix):]))
		}
		return strings.TrimSpace(rest[len(prefix):])
	}
	return ""
}

func formatAPIErrorLines(attempts, statusCode int, payload any, requestID ...string) string {
	var body codexAPIErrorBody
	var rid string
	if len(requestID) > 0 {
		rid = requestID[0]
	}
	switch p := payload.(type) {
	case codexAPIErrorBody:
		body = p
	case string:
		if b, id, ok := extractAPIErrorPayload(p); ok {
			body = b
			if rid == "" {
				rid = id
			}
		} else {
			body.Message = strings.TrimSpace(p)
		}
	default:
		return fmt.Sprint(payload)
	}
	displayMsg := displayAPIErrorMessage(body.Type, body.Message, body.Detail)
	var lines []string
	if summary := apiErrorSummary(statusCode, body.Type, displayMsg); summary != "" {
		lines = append(lines, "summary: "+summary)
	}
	if attempts > 0 {
		lines = append(lines, fmt.Sprintf("attempts: %d", attempts))
	}
	if statusCode > 0 {
		lines = append(lines, fmt.Sprintf("HTTP: %d", statusCode))
	} else if s, ok := payload.(string); ok {
		if statusLine := extractHTTPStatusLine(s); statusLine != "" {
			lines = append(lines, "HTTP: "+statusLine)
		}
	}
	if body.Type != "" {
		lines = append(lines, "type: "+body.Type)
	}
	if displayMsg != "" {
		lines = append(lines, "message: "+displayMsg)
	}
	if body.PlanType != "" {
		lines = append(lines, "plan: "+body.PlanType)
	}
	if reset := formatAPIResetLine(body); reset != "" {
		lines = append(lines, reset)
	}
	if rid != "" {
		lines = append(lines, "request_id: "+rid)
	}
	if hint := apiErrorResolutionHint(attempts, statusCode, body.Type, displayMsg); hint != "" {
		lines = append(lines, "hint: "+hint)
	}
	if len(lines) == 0 {
		return fmt.Sprint(payload)
	}
	return strings.Join(lines, "\n")
}

func apiErrorSummary(statusCode int, errType, message string) string {
	switch errType {
	case "rate_limit_error", "usage_limit_reached":
		return "rate limit reached"
	case "not_found_error":
		if strings.HasPrefix(strings.TrimSpace(message), "model:") {
			return "model not found"
		}
		return "resource not found"
	case "authentication_error":
		return "authentication failed"
	case "permission_error":
		return "permission denied"
	case "overloaded_error":
		return "provider overloaded"
	case "invalid_request_error":
		return "invalid request"
	}
	switch statusCode {
	case 401:
		return "authentication failed"
	case 403:
		return "permission denied"
	case 404:
		return "not found"
	case 429:
		return "rate limit reached"
	case 502, 503, 504:
		return "provider unavailable"
	}
	return ""
}

func displayAPIErrorMessage(errType, message, detail string) string {
	msg := strings.TrimSpace(message)
	if msg == "" {
		msg = strings.TrimSpace(detail)
	}
	if msg != "" && !strings.EqualFold(msg, "error") {
		return msg
	}
	switch errType {
	case "rate_limit_error":
		return "too many requests"
	case "not_found_error":
		return "resource not found"
	case "authentication_error":
		return "invalid or missing API credentials"
	case "permission_error":
		return "permission denied for this request"
	case "overloaded_error":
		return "provider is overloaded"
	case "invalid_request_error":
		return "invalid API request"
	default:
		return msg
	}
}

func apiErrorResolutionHint(attempts, statusCode int, errType, message string) string {
	msg := strings.TrimSpace(message)
	if strings.HasPrefix(msg, "model:") {
		model := strings.TrimSpace(strings.TrimPrefix(msg, "model:"))
		if model == "" {
			return "Run /models and pick an active model for this provider."
		}
		if strings.Contains(model, "20250514") {
			return fmt.Sprintf("Model %q was retired (2026-06-15). Run /models and select a current model such as claude-sonnet-4-6.", model)
		}
		return fmt.Sprintf("Model %q is unknown or no longer available. Run /models and pick a current model.", model)
	}
	switch errType {
	case "rate_limit_error", "usage_limit_reached":
		var parts []string
		parts = append(parts, "Wait 1–2 minutes before sending another message.")
		if attempts >= 3 {
			parts = append(parts, "Solomon paused this provider for ~60s after repeated failures.")
		}
		parts = append(parts, "If you use a Claude Code OAuth token (sk-ant-oat…), limits are often stricter — prefer a console.anthropic.com API key via /connect.")
		parts = append(parts, "Try /reasoning off or a lighter model via /models.")
		return strings.Join(parts, " ")
	case "not_found_error":
		if statusCode == 404 {
			return "Check provider base URL and model name with /models."
		}
		return "Verify the resource exists and is still supported by the provider."
	case "authentication_error":
		return "Check the API key or OAuth login for this provider (/connect)."
	case "permission_error":
		return "This API key or account cannot access the requested model. Try /models or upgrade the provider plan."
	case "overloaded_error":
		return "Retry in a few minutes or switch to another provider with /models."
	case "invalid_request_error":
		return "Check model name, reasoning settings (/reasoning), and provider configuration."
	}
	switch statusCode {
	case 401:
		return "Check the API key or OAuth login for this provider (/connect)."
	case 403:
		return "This account cannot access the requested model. Try /models or another provider."
	case 404:
		return "The endpoint or model was not found. Run /models to refresh available models."
	case 429:
		hint := "Wait before retrying. If limits persist, switch provider with /models or use a dedicated API key."
		if attempts >= 3 {
			hint += " Solomon paused this provider for ~60s after repeated failures."
		}
		return hint
	case 502, 503, 504:
		return "The provider is temporarily down. Retry shortly or switch provider with /models."
	}
	if strings.Contains(strings.ToLower(msg), "no such host") || strings.Contains(strings.ToLower(msg), "connection refused") {
		return "Cannot reach the provider. Check internet connectivity, base URL, and that local servers (LM Studio, Cursor sidecar) are running."
	}
	return ""
}

func formatHTTPRequestError(msg string) (string, bool) {
	quote := strings.Index(msg, `"`)
	if quote <= 0 {
		return "", false
	}
	method := strings.TrimSpace(msg[:quote])
	if method == "" {
		return "", false
	}
	rest := msg[quote:]
	end := strings.Index(rest[1:], `"`)
	if end < 0 {
		return "", false
	}
	url := rest[1 : 1+end]
	tail := strings.TrimSpace(rest[1+end+1:])
	if !strings.HasPrefix(tail, ":") {
		return "", false
	}
	detail := strings.TrimSpace(tail[1:])
	if detail == "" {
		return "", false
	}
	lines := []string{
		"request: " + strings.ToUpper(method) + " " + url,
		"error: " + detail,
	}
	if hint := apiErrorResolutionHint(0, 0, "", detail); hint != "" {
		lines = append(lines, "hint: "+hint)
	}
	return strings.Join(lines, "\n"), true
}

func formatModelsAPIPlainError(msg string) (string, bool) {
	const prefix = "models API: "
	if !strings.HasPrefix(msg, prefix) {
		return "", false
	}
	if strings.Contains(msg, "{") {
		return "", false
	}
	rest := strings.TrimSpace(msg[len(prefix):])
	if rest == "" {
		return "", false
	}
	lines := []string{"source: models API"}
	statusCode := 0
	if i := strings.Index(rest, ": "); i >= 0 {
		statusLine := strings.TrimSpace(rest[:i])
		lines = append(lines, "HTTP: "+statusLine)
		statusCode = statusCodeFromMessage(statusLine)
		if tail := strings.TrimSpace(rest[i+2:]); tail != "" {
			lines = append(lines, "message: "+tail)
		}
	} else {
		lines = append(lines, "HTTP: "+rest)
	}
	lines = append(lines, "hint: Check provider credentials (/connect), base URL, and network connectivity.")
	if statusCode == 401 || statusCode == 403 {
		lines[len(lines)-1] = "hint: Authentication failed while listing models. Re-run /connect or update the API key in config.toml."
	}
	return strings.Join(lines, "\n"), true
}

func extractHTTPStatusLine(msg string) string {
	before := msg
	if i := strings.Index(msg, "{"); i >= 0 {
		before = strings.TrimSpace(msg[:i])
	}
	if li := strings.LastIndex(before, ": "); li >= 0 {
		tail := strings.TrimSpace(before[li+2:])
		if len(tail) >= 3 && tail[0] >= '0' && tail[0] <= '9' {
			return tail
		}
	}
	return ""
}

func formatAPIResetLine(body codexAPIErrorBody) string {
	if body.ResetsAt <= 0 && body.ResetsInSeconds <= 0 {
		return ""
	}
	var when string
	if body.ResetsAt > 0 {
		when = time.Unix(body.ResetsAt, 0).Local().Format("2006-01-02 15:04:05")
	}
	sec := body.ResetsInSeconds
	if sec <= 0 && body.ResetsAt > 0 {
		sec = int64(time.Until(time.Unix(body.ResetsAt, 0)).Seconds())
		if sec < 0 {
			sec = 0
		}
	}
	switch {
	case when != "" && sec > 0:
		return fmt.Sprintf("reset: %s (in %d seconds)", when, sec)
	case when != "":
		return "reset: " + when
	case sec > 0:
		return fmt.Sprintf("reset: in %d seconds", sec)
	default:
		return ""
	}
}
