package cursor

import (
	"sort"
	"strings"
	"sync"
)

func DedupeVariantIDs(ids []string) []string {
	flags := make([]ModelFlag, 0, len(ids))
	for _, id := range ids {
		fast, thinking, levels := cursorVariantBits(id)
		flags = append(flags, ModelFlag{ID: id, Fast: fast, Thinking: thinking, ThinkingLevels: levels})
	}
	return DedupeVariantFlags(flags).IDs
}

type ModelCaps struct {
	IDs            []string
	Fast           []string
	ThinkingToggle []string
	ThinkingLevels []string
	Variants       []string
}

func DedupeVariantFlags(items []ModelFlag) ModelCaps {
	best := map[string]ModelFlag{}
	order := make([]string, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" || isCursorVariantLabel(id) {
			continue
		}
		base := cursorBaseModelID(id)
		if base == "" {
			continue
		}
		key := strings.ToLower(base)
		fast, thinking, levels := cursorVariantBits(id)
		item.Fast = item.Fast || fast
		item.Thinking = item.Thinking || thinking
		item.ThinkingLevels = item.ThinkingLevels || levels
		prev, exists := best[key]
		if !exists {
			best[key] = ModelFlag{ID: id, Fast: item.Fast, Thinking: item.Thinking, ThinkingLevels: item.ThinkingLevels}
			order = append(order, key)
			continue
		}
		if strings.EqualFold(id, base) && !strings.EqualFold(prev.ID, base) {
			prev.ID = id
		}
		prev.Fast = prev.Fast || item.Fast
		prev.Thinking = prev.Thinking || item.Thinking
		prev.ThinkingLevels = prev.ThinkingLevels || item.ThinkingLevels
		best[key] = prev
	}
	caps := ModelCaps{}
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" || isCursorVariantLabel(id) {
			continue
		}
		raw := strings.TrimSpace(item.Raw)
		if raw == "" {
			raw = id
		}
		caps.Variants = append(caps.Variants, raw)
	}
	for _, key := range order {
		item := best[key]
		id := item.ID
		if !strings.EqualFold(id, key) {
			id = cursorBaseModelID(id)
		}
		caps.IDs = append(caps.IDs, id)
		if item.Fast {
			caps.Fast = append(caps.Fast, id)
		}
		if item.ThinkingLevels {
			caps.ThinkingLevels = append(caps.ThinkingLevels, id)
		} else if item.Thinking {
			caps.ThinkingToggle = append(caps.ThinkingToggle, id)
		}
	}
	return caps
}

var (
	fastMu         sync.Mutex
	fastLoaded     bool
	fastAllowed    = map[string]bool{}
	thinkingToggle = map[string]bool{}
	thinkingLevels = map[string]bool{}
	knownVariants  []string
)

func SetModelCaps(caps ModelCaps) {
	fastMu.Lock()
	defer fastMu.Unlock()
	fastLoaded = true
	fastAllowed = map[string]bool{}
	thinkingToggle = map[string]bool{}
	thinkingLevels = map[string]bool{}
	knownVariants = append([]string(nil), caps.Variants...)
	for _, id := range caps.Fast {
		if id = strings.ToLower(strings.TrimSpace(id)); id != "" {
			fastAllowed[id] = true
		}
	}
	for _, id := range caps.ThinkingToggle {
		if id = strings.ToLower(strings.TrimSpace(id)); id != "" {
			thinkingToggle[id] = true
		}
	}
	for _, id := range caps.ThinkingLevels {
		if id = strings.ToLower(strings.TrimSpace(id)); id != "" {
			thinkingLevels[id] = true
		}
	}
}

func FastModelIDs() []string {
	return capMapKeys(fastAllowed)
}

func ModelSupportsFast(id string) bool {
	fastMu.Lock()
	defer fastMu.Unlock()
	if !fastLoaded {
		return true
	}
	return fastAllowed[strings.ToLower(cursorBaseModelID(id))]
}

func ThinkingMode(id string) string {
	fastMu.Lock()
	defer fastMu.Unlock()
	if !fastLoaded {
		return ""
	}
	key := strings.ToLower(cursorBaseModelID(id))
	if thinkingLevels[key] {
		return "levels"
	}
	if thinkingToggle[key] {
		return "toggle"
	}
	return "none"
}

func ThinkingToggleIDs() []string {
	return capMapKeys(thinkingToggle)
}

func ThinkingLevelIDs() []string {
	return capMapKeys(thinkingLevels)
}

func capMapKeys(values map[string]bool) []string {
	fastMu.Lock()
	defer fastMu.Unlock()
	if !fastLoaded {
		return nil
	}
	out := make([]string, 0, len(values))
	for id := range values {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func isCursorVariantLabel(id string) bool {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "fast", "thinking", "max", "default", "high", "low", "medium", "mid", "xhigh", "extra-high", "auto-fast", "none":
		return true
	default:
		return false
	}
}

func cursorVariantBits(id string) (fast, thinking, levels bool) {
	low := strings.ToLower(strings.TrimSpace(id))
	fast = strings.Contains(low, "fast")
	thinking = strings.Contains(low, "thinking")
	for _, token := range []string{"thinking-low", "thinking-medium", "thinking-high", "-xhigh", "-extra-high"} {
		if strings.Contains(low, token) {
			return fast, true, true
		}
	}
	for _, suf := range []string{"-high", "-low", "-medium", "-mid"} {
		if strings.HasSuffix(low, suf) {
			return fast, true, true
		}
	}
	return fast, thinking, false
}

func pickCursorVariant(ids []string, base, level string, fast bool) string {
	fallback := base
	for _, id := range ids {
		if strings.EqualFold(id, base) {
			fallback = id
		}
	}
	search := level
	if search == "" {
		search = "none"
	}
	for _, wantFast := range []bool{fast, false} {
		if !fast && wantFast {
			continue
		}
		for _, id := range ids {
			gotLevel, gotFast := cursorVariantEffort(id, base)
			if gotLevel == search && gotFast == wantFast {
				return id
			}
		}
	}
	return fallback
}

func cursorVariantEffort(id, base string) (level string, fast bool) {
	rest := strings.TrimPrefix(strings.ToLower(id), strings.ToLower(base))
	rest = strings.Trim(rest, "-")
	fast = strings.Contains(rest, "fast")
	rest = strings.ReplaceAll(rest, "fast", "")
	rest = strings.Trim(rest, "-")
	rest = strings.TrimPrefix(rest, "thinking-")
	rest = strings.TrimPrefix(rest, "thinking")
	return strings.Trim(rest, "-"), fast
}

func cursorBaseModelID(id string) string {
	id = strings.TrimSpace(id)
	prefix := ""
	if len(id) >= 7 && strings.EqualFold(id[:7], "cursor-") {
		prefix = id[:7]
		id = id[7:]
	}
	if i := strings.IndexAny(id, "[(:"); i > 0 {
		id = strings.TrimSpace(id[:i])
	}
	for {
		n := len(id)
		low := strings.ToLower(id)
		for _, suf := range []string{
			"-thinking-medium", "-thinking-low", "-thinking-high", "-thinking",
			"-fast", "-max-mode", "-extra-high", "-xhigh",
			"-high", "-medium", "-mid", "-low", "-max", "-none",
		} {
			if strings.HasSuffix(low, suf) {
				id = strings.TrimRight(id[:len(id)-len(suf)], "-_")
				break
			}
		}
		if len(id) == n {
			return prefix + id
		}
	}
}
