//go:build automatic_role_scores

package roles

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	modelParamSizeRe  = regexp.MustCompile(`-\d+b(?:\b|[^a-z])`)
	modelDateSuffixRe = regexp.MustCompile(`-\d{8}$`)
)

func CompareModelRecency(a, b string) int {
	sa, sb := modelRecencyScore(a), modelRecencyScore(b)
	if sa != sb {
		return sa - sb
	}
	return strings.Compare(NormalizeModelID(a), NormalizeModelID(b))
}

func ModelRecencyScore(id string) int {
	return modelRecencyScore(id)
}

func modelRecencyScore(id string) int {
	id = NormalizeModelID(id)
	if id == "" {
		return 0
	}
	id = modelParamSizeRe.ReplaceAllString(id, "")
	id = modelDateSuffixRe.ReplaceAllString(id, "")
	if at := strings.Index(id, "@"); at > 0 {
		id = id[:at]
	}
	score := parseKnownModelVersionScore(id)
	score += modelRecencyTierBonus(id)
	return score
}

func parseKnownModelVersionScore(id string) int {
	for _, prefix := range []string{
		"gpt-", "claude-", "gemini-", "grok-", "composer-", "kimi-", "glm-",
		"qwen", "deepseek-", "llama-",
	} {
		if strings.HasPrefix(id, prefix) {
			rest := strings.TrimPrefix(id, prefix)
			if strings.HasPrefix(prefix, "qwen") {
				rest = strings.TrimPrefix(id, "qwen")
				rest = strings.TrimPrefix(rest, "-")
				if dot := strings.Index(rest, "."); dot > 0 {
					rest = rest[:dot] + "-" + rest[dot+1:]
				}
			}
			parts := strings.Split(rest, "-")
			if s := dashedVersionScore(parts); s > 0 {
				return s
			}
		}
	}
	return 0
}

func dashedVersionScore(parts []string) int {
	if len(parts) == 0 {
		return 0
	}
	version := strings.Split(parts[0], ".")
	major, err := strconv.Atoi(version[0])
	if err != nil || major <= 0 || major > 20 {
		return 0
	}
	score := major * 100
	minorText := ""
	if len(version) > 1 {
		minorText = version[1]
	} else if len(parts) >= 2 {
		minorText = parts[1]
	}
	if minor, err := strconv.Atoi(minorText); err == nil && minor >= 0 && minor <= 99 {
		score += minor
	}
	return score
}

func modelRecencyTierBonus(id string) int {
	switch {
	case strings.Contains(id, "opus"):
		return 50
	case strings.Contains(id, "sonnet"):
		return 40
	case strings.Contains(id, "pro"):
		return 35
	case strings.Contains(id, "r1"):
		return 45
	case strings.Contains(id, "composer"):
		return 42
	case strings.Contains(id, "codex"):
		return 38
	case strings.Contains(id, "haiku"):
		return 20
	case strings.Contains(id, "mini"):
		return -10
	case strings.Contains(id, "nano"):
		return -15
	case strings.Contains(id, "micro"):
		return -20
	default:
		return 0
	}
}
