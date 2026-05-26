package cursor

import (
	"sort"
	"strconv"
	"strings"
)

const DefaultModelID = "composer-2.5"

type cursorLab int

const (
	labComposer cursorLab = iota
	labOpenAI
	labAnthropic
	labXAI
	labKimi
	labAuto
	labOther
)

func FilterModelIDs(ids []string) []string {
	if len(ids) == 0 {
		return []string{DefaultModelID, "auto"}
	}
	byLab := make(map[cursorLab][]string)
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		lab := classifyCursorLab(id)
		if lab == labOther {
			continue
		}
		byLab[lab] = append(byLab[lab], id)
	}
	order := []cursorLab{labComposer, labOpenAI, labAnthropic, labXAI, labKimi, labAuto}
	var out []string
	for _, lab := range order {
		if id := pickLabFlagship(lab, byLab[lab]); id != "" {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return []string{DefaultModelID, "auto"}
	}
	return ensureAutoLast(out)
}

func OrderModelIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	byLab := make(map[cursorLab][]string)
	var other []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		lab := classifyCursorLab(id)
		if lab == labOther {
			other = append(other, id)
			continue
		}
		byLab[lab] = append(byLab[lab], id)
	}
	labOrder := []cursorLab{labComposer, labOpenAI, labAnthropic, labXAI, labKimi, labAuto}
	var out []string
	for _, lab := range labOrder {
		out = append(out, sortLabModelIDs(lab, byLab[lab])...)
	}
	out = append(out, other...)
	if len(out) == 0 {
		return out
	}
	for _, id := range out {
		if strings.EqualFold(id, "auto") {
			return ensureAutoLast(out)
		}
	}
	return out
}

func sortLabModelIDs(lab cursorLab, ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	type item struct {
		id string
		sc flagshipScore
	}
	items := make([]item, 0, len(ids))
	for _, id := range ids {
		sc := scoreLabModel(lab, id)
		items = append(items, item{id: id, sc: sc})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].sc.ok != items[j].sc.ok {
			return items[i].sc.ok
		}
		if !items[i].sc.ok {
			return items[i].id < items[j].id
		}
		return flagshipBetter(items[i].sc, items[j].sc)
	})
	out := make([]string, len(items))
	for i := range items {
		out[i] = items[i].id
	}
	return out
}

func ensureAutoLast(out []string) []string {
	filtered := make([]string, 0, len(out))
	for _, id := range out {
		if !strings.EqualFold(id, "auto") {
			filtered = append(filtered, id)
		}
	}
	return append(filtered, "auto")
}

func classifyCursorLab(id string) cursorLab {
	m := strings.ToLower(strings.TrimSpace(id))
	switch {
	case m == "auto":
		return labAuto
	case strings.HasPrefix(m, "composer"):
		return labComposer
	case strings.HasPrefix(m, "gpt"):
		return labOpenAI
	case strings.Contains(m, "claude"):
		return labAnthropic
	case strings.Contains(m, "grok"):
		return labXAI
	case strings.Contains(m, "kimi"):
		return labKimi
	default:
		return labOther
	}
}

func pickLabFlagship(lab cursorLab, ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	if lab == labAuto {
		for _, id := range ids {
			if strings.EqualFold(id, "auto") {
				return id
			}
		}
		return ""
	}
	var best string
	var bestSc flagshipScore
	for _, id := range ids {
		sc := scoreLabModel(lab, id)
		if !sc.ok {
			continue
		}
		if best == "" || flagshipBetter(sc, bestSc) {
			best = id
			bestSc = sc
		}
	}
	return best
}

type flagshipScore struct {
	ver  []int
	tier int
	ok   bool
}

func scoreLabModel(lab cursorLab, id string) flagshipScore {
	switch lab {
	case labComposer:
		return scoreComposer(id)
	case labOpenAI:
		return scoreGPT(id)
	case labAnthropic:
		return scoreAnthropic(id)
	case labXAI:
		return scoreGrok(id)
	case labKimi:
		return scoreKimi(id)
	default:
		return flagshipScore{}
	}
}

func flagshipBetter(a, b flagshipScore) bool {
	if c := compareVersionKeys(a.ver, b.ver); c != 0 {
		return c > 0
	}
	return a.tier > b.tier
}

func scoreComposer(id string) flagshipScore {
	m := strings.ToLower(strings.TrimSpace(id))
	if !strings.HasPrefix(m, "composer") {
		return flagshipScore{}
	}
	rest := strings.TrimPrefix(m, "composer")
	rest = strings.TrimPrefix(rest, "-")
	if rest == "" {
		return flagshipScore{}
	}
	parts := strings.Split(rest, "-")
	ver := parseVersionSegment(parts[0])
	if len(ver) == 0 {
		return flagshipScore{}
	}
	return flagshipScore{ver: ver, tier: composerVariantTier(parts[1:]), ok: true}
}

func scoreGPT(id string) flagshipScore {
	m := strings.ToLower(strings.TrimSpace(id))
	if !strings.HasPrefix(m, "gpt") {
		return flagshipScore{}
	}
	for _, p := range []string{"gpt-image", "gpt-realtime", "gpt-audio"} {
		if strings.HasPrefix(m, p) {
			return flagshipScore{}
		}
	}
	rest := strings.TrimPrefix(m, "gpt-")
	parts := strings.Split(rest, "-")
	ver := parseVersionSegment(parts[0])
	if len(ver) == 0 {
		return flagshipScore{}
	}
	return flagshipScore{ver: ver, tier: gptVariantTier(parts[1:]), ok: true}
}

func scoreAnthropic(id string) flagshipScore {
	m := strings.ToLower(strings.TrimSpace(id))
	if !strings.Contains(m, "claude") {
		return flagshipScore{}
	}
	rest := strings.TrimPrefix(m, "claude")
	rest = strings.TrimPrefix(rest, "-")
	parts := strings.Split(rest, "-")
	ver := versionKeyFromParts(parts)
	tier := anthropicModelLineTier(m)
	if strings.Contains(m, "opus") {
		if t := anthropicVariantTier(parts); t > 0 {
			tier = t
		}
	}
	return flagshipScore{ver: ver, tier: tier, ok: true}
}

func anthropicModelLineTier(m string) int {
	switch {
	case strings.Contains(m, "opus"):
		return 100
	case strings.Contains(m, "sonnet"):
		return 75
	case strings.Contains(m, "haiku"):
		return 50
	default:
		return 60
	}
}

func scoreGrok(id string) flagshipScore {
	m := strings.ToLower(strings.TrimSpace(id))
	if !strings.Contains(m, "grok") || strings.Contains(m, "build") {
		return flagshipScore{}
	}
	rest := strings.TrimPrefix(m, "grok-")
	parts := strings.Split(rest, "-")
	ver := parseVersionSegment(parts[0])
	if len(ver) == 0 {
		return flagshipScore{}
	}
	return flagshipScore{ver: ver, tier: grokVariantTier(parts[1:]), ok: true}
}

func scoreKimi(id string) flagshipScore {
	m := strings.ToLower(strings.TrimSpace(id))
	if !strings.Contains(m, "kimi") {
		return flagshipScore{}
	}
	idx := strings.Index(m, "kimi")
	rest := strings.TrimPrefix(m[idx+4:], "-")
	rest = strings.TrimPrefix(rest, "k")
	ver := parseVersionSegment(rest)
	if len(ver) == 0 {
		parts := strings.Split(m, "-")
		for i, p := range parts {
			if p != "kimi" || i+1 >= len(parts) {
				continue
			}
			seg := strings.TrimPrefix(parts[i+1], "k")
			ver = parseVersionSegment(seg)
			break
		}
	}
	if len(ver) == 0 {
		return flagshipScore{}
	}
	return flagshipScore{ver: ver, tier: 100, ok: true}
}

func composerVariantTier(suffix []string) int {
	if len(suffix) == 0 {
		return 100
	}
	s := strings.Join(suffix, "-")
	switch {
	case strings.Contains(s, "fast"):
		return 40
	case strings.Contains(s, "beta"):
		return 60
	default:
		return 80
	}
}

func gptVariantTier(suffix []string) int {
	if len(suffix) == 0 {
		return 100
	}
	s := strings.Join(suffix, "-")
	switch {
	case strings.Contains(s, "mini"):
		return 30
	case strings.Contains(s, "nano"):
		return 25
	case strings.Contains(s, "codex"):
		return 50
	case strings.Contains(s, "pro"):
		return 20
	case strings.Contains(s, "medium"):
		return 85
	default:
		return 70
	}
}

func anthropicVariantTier(parts []string) int {
	if len(parts) <= 1 {
		return 80
	}
	s := strings.Join(parts[1:], "-")
	switch {
	case strings.Contains(s, "thinking-medium"):
		return 100
	case strings.Contains(s, "thinking"):
		return 90
	default:
		return 70
	}
}

func grokVariantTier(suffix []string) int {
	if len(suffix) == 0 {
		return 100
	}
	return 70
}

func versionKeyFromParts(parts []string) []int {
	var key []int
	for _, p := range parts {
		if p == "" {
			continue
		}
		if isAnthropicSuffixPart(p) {
			break
		}
		seg := parseVersionSegment(p)
		if len(seg) == 0 {
			break
		}
		key = append(key, seg...)
	}
	return key
}

func isAnthropicSuffixPart(p string) bool {
	switch p {
	case "thinking", "medium", "fast", "high", "low", "haiku", "sonnet", "opus":
		return true
	default:
		return strings.Contains(p, "thinking")
	}
}

func compareVersionKeys(a, b []int) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		var xa, xb int
		if i < len(a) {
			xa = a[i]
		}
		if i < len(b) {
			xb = b[i]
		}
		if xa != xb {
			return xa - xb
		}
	}
	return 0
}

func parseVersionSegment(ver string) []int {
	ver = strings.TrimSpace(ver)
	if ver == "" {
		return nil
	}
	if strings.Contains(ver, ".") {
		var key []int
		for _, p := range strings.Split(ver, ".") {
			n, rest := parseLeadingDigits(p)
			if n < 0 {
				continue
			}
			key = append(key, n)
			if rest != "" {
				key = append(key, int(rest[0]))
			}
		}
		return key
	}
	n, rest := parseLeadingDigits(ver)
	if n < 0 {
		return nil
	}
	key := []int{n}
	if rest != "" {
		key = append(key, int(rest[0]))
	}
	return key
}

func parseLeadingDigits(s string) (int, string) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return -1, s
	}
	n, _ := strconv.Atoi(s[:i])
	return n, s[i:]
}
