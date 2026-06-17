package research

import (
	"errors"
	"strings"
)

var ErrPausedLLM = errors.New("research paused: LLM unavailable")

func pauseLLMError(detail string) error {
	detail = compactResearchError(detail)
	if detail == "" {
		return ErrPausedLLM
	}
	return errors.Join(ErrPausedLLM, errors.New(detail))
}

func FormatResearchError(msg string) string {
	return compactResearchError(msg)
}

func compactResearchError(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}
	if j := strings.Index(msg, "{"); j >= 0 {
		if obj := parseJSONObject(msg[j:]); obj != nil {
			if m, ok := obj["message"].(string); ok && strings.TrimSpace(m) != "" {
				return strings.Join(strings.Fields(m), " ")
			}
		}
	}
	msg = strings.Join(strings.Fields(msg), " ")
	const max = 160
	if len(msg) > max {
		return msg[:max-3] + "..."
	}
	return msg
}
