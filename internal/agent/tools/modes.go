package tools

import "strings"

func NormalizeMode(m string) string {
	return normalizeMode(m)
}

func normalizeMode(m string) string {
	switch strings.TrimSpace(strings.ToLower(m)) {
	case "chat":
		return "chat"
	case "plan":
		return "plan"
	case "build":
		return "build"
	default:
		return "agent"
	}
}

func EffectiveSurfaceMode(m string) string {
	switch NormalizeMode(m) {
	case "chat":
		return "chat"
	case "plan", "build":
		return "agent"
	default:
		return "agent"
	}
}
