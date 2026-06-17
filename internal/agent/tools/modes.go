package tools

import "strings"

func NormalizeMode(m string) string {
	return normalizeMode(m)
}

func normalizeMode(m string) string {
	switch strings.TrimSpace(strings.ToLower(m)) {
	case "chat":
		return "chat"
	default:
		return "agent"
	}
}

func EffectiveSurfaceMode(m string) string {
	return normalizeMode(m)
}

func LegacyDeferredToolNames() []string {
	return AgentDeferredToolNames()
}
