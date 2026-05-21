package llm

import "strings"

func anthropicMessagesURL(base string) string {
	base = strings.TrimSpace(base)
	base = strings.TrimSuffix(base, "/")
	if strings.HasSuffix(base, "/v1/messages") {
		return base
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/messages"
	}
	return base + "/v1/messages"
}
