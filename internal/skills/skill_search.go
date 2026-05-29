package skills

import (
	"fmt"
	"math"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

const bm25k1 = 1.2
const bm25b = 0.75

func tokenizeSearchText(s string) []string {
	s = strings.ToLower(s)
	var cur strings.Builder
	var out []string
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		out = append(out, cur.String())
		cur.Reset()
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

type bm25Corpus struct {
	docs     [][]string
	docLen   []int
	avgdl    float64
	n        int
	df       map[string]int
	termFreq []map[string]int
}

func newBM25Corpus(documents []string) *bm25Corpus {
	n := len(documents)
	if n == 0 {
		return &bm25Corpus{n: 0, df: map[string]int{}}
	}
	c := &bm25Corpus{
		n:        n,
		df:       map[string]int{},
		docs:     make([][]string, n),
		docLen:   make([]int, n),
		termFreq: make([]map[string]int, n),
	}
	sumdl := 0
	for i, raw := range documents {
		toks := tokenizeSearchText(raw)
		c.docs[i] = toks
		c.docLen[i] = len(toks)
		sumdl += len(toks)
		f := map[string]int{}
		seen := map[string]struct{}{}
		for _, t := range toks {
			f[t]++
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				c.df[t]++
			}
		}
		c.termFreq[i] = f
	}
	c.avgdl = float64(sumdl) / float64(n)
	if c.avgdl == 0 {
		c.avgdl = 1
	}
	return c
}

func (c *bm25Corpus) scoreDoc(docI int, qterms []string) float64 {
	if docI < 0 || docI >= c.n {
		return 0
	}
	dl := float64(c.docLen[docI])
	f := c.termFreq[docI]
	score := 0.0
	for _, t := range qterms {
		ft := f[t]
		if ft == 0 {
			continue
		}
		df := c.df[t]
		idf := math.Log(1 + (float64(c.n)-float64(df)+0.5)/(float64(df)+0.5))
		denom := float64(ft) + bm25k1*(1-bm25b+bm25b*dl/c.avgdl)
		score += idf * (float64(ft) * (bm25k1 + 1)) / denom
	}
	return score
}

func (c *bm25Corpus) scoreCeiling(qterms []string) float64 {
	var sum float64
	for _, t := range qterms {
		df, ok := c.df[t]
		if !ok || df == 0 {
			continue
		}
		idf := math.Log(1 + (float64(c.n)-float64(df)+0.5)/(float64(df)+0.5))
		sum += idf * (bm25k1 + 1)
	}
	return sum
}

func (c *bm25Corpus) bestDocAndScore(qterms []string) (int, float64) {
	if len(qterms) == 0 || c.n == 0 {
		return 0, 0
	}
	besti := 0
	best := -1.0
	for i := 0; i < c.n; i++ {
		s := c.scoreDoc(i, qterms)
		if s > best {
			best = s
			besti = i
		}
	}
	return besti, best
}

func trySkillSearchPhase(binds []SkillSlashBinding, docTexts []string, qterms []string, minNorm float64) (idx int, normalized float64, ok bool) {
	if len(binds) != len(docTexts) {
		return 0, 0, false
	}
	corp := newBM25Corpus(docTexts)
	idx, raw := corp.bestDocAndScore(qterms)
	if raw <= 0 {
		return 0, 0, false
	}
	ceil := corp.scoreCeiling(qterms)
	if ceil <= 0 {
		return 0, 0, false
	}
	normalized = raw / ceil
	if normalized > 1 {
		normalized = 1
	}
	if minNorm <= 0 {
		return idx, normalized, true
	}
	if normalized < minNorm {
		return idx, normalized, false
	}
	return idx, normalized, true
}

func skillSearchFullFileText(e *SkillEntry) string {
	p := strings.TrimSpace(e.SkillMdPath)
	if p == "" {
		return skillSearchDescriptionOnly(e)
	}
	b, err := os.ReadFile(p)
	if err != nil || len(b) == 0 {
		return skillSearchDescriptionOnly(e)
	}
	return string(b)
}

func skillSearchDescriptionOnly(e *SkillEntry) string {
	d := DescriptionFromFrontMatter(e.FrontMatter)
	if strings.TrimSpace(d) != "" {
		return d
	}
	return strings.TrimSpace(e.Name)
}

type SkillSearchHit struct {
	Name        string  `json:"name"`
	Slash       string  `json:"slash"`
	Description string  `json:"description"`
	Score       float64 `json:"score"`
}

func SearchBestInstalledSkill(query string, projHex, projRoot string, minNormalized float64) (*SkillSearchHit, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, fmt.Errorf("searchSkill: empty query")
	}
	qterms := tokenizeSearchText(q)
	if len(qterms) == 0 {
		return nil, fmt.Errorf("searchSkill: query has no searchable terms")
	}
	regPath, err := paths.SkillsRegistryPath()
	if err != nil {
		return nil, err
	}
	reg, err := LoadRegistry(regPath)
	if err != nil {
		return nil, err
	}
	refs := OrderedSkillRefs(reg, projHex, projRoot)
	binds := AssignSkillSlashCommands(refs)
	if len(binds) == 0 {
		return nil, fmt.Errorf("no skills installed")
	}
	textsDesc := make([]string, len(binds))
	for i := range binds {
		textsDesc[i] = skillSearchDescriptionOnly(&binds[i].Entry)
	}
	idx, norm, ok := trySkillSearchPhase(binds, textsDesc, qterms, minNormalized)
	if !ok {
		textsFull := make([]string, len(binds))
		for i := range binds {
			textsFull[i] = skillSearchFullFileText(&binds[i].Entry)
		}
		idx, norm, ok = trySkillSearchPhase(binds, textsFull, qterms, minNormalized)
	}
	if !ok {
		if minNormalized > 0 {
			return nil, fmt.Errorf("searchSkill: no skill reaches minimum normalized score %.4f for query %q", minNormalized, q)
		}
		return nil, fmt.Errorf("searchSkill: no matching skill for query %q", q)
	}
	b := binds[idx]
	desc := DescriptionFromFrontMatter(b.Entry.FrontMatter)
	return &SkillSearchHit{
		Name:        strings.TrimSpace(b.Entry.Name),
		Slash:       b.Slash,
		Description: desc,
		Score:       norm,
	}, nil
}

func ResolveSkillForLoad(raw string, projHex, projRoot string) (*SkillEntry, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", fmt.Errorf("empty skill name")
	}
	binds, err := loadSkillBindings(projHex, projRoot)
	if err != nil {
		return nil, "", err
	}
	return resolveSkillFromBindings(binds, raw)
}

func loadSkillBindings(projHex, projRoot string) ([]SkillSlashBinding, error) {
	regPath, err := paths.SkillsRegistryPath()
	if err != nil {
		return nil, err
	}
	reg, err := LoadRegistry(regPath)
	if err != nil {
		return nil, err
	}
	refs := OrderedSkillRefs(reg, projHex, projRoot)
	return AssignSkillSlashCommands(refs), nil
}

func resolveSkillFromBindings(binds []SkillSlashBinding, raw string) (*SkillEntry, string, error) {
	slashKey := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(raw), "/"))
	for i := range binds {
		if binds[i].Slash == slashKey {
			e := binds[i].Entry
			return &e, binds[i].Slash, nil
		}
	}
	for i := range binds {
		if nameMatches(binds[i].Entry.Name, raw) {
			e := binds[i].Entry
			return &e, binds[i].Slash, nil
		}
	}
	return nil, "", fmt.Errorf("skill not found: %q", raw)
}

func ResolveForcedSkillCommand(raw, projHex, projRoot string) (*SkillEntry, string, string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(strings.ToLower(raw), "/skill:") {
		return nil, "", "", fmt.Errorf("invalid forced skill command")
	}
	rest := strings.TrimSpace(raw[len("/skill:"):])
	if rest == "" {
		return nil, "", "", fmt.Errorf("empty skill name")
	}
	binds, err := loadSkillBindings(projHex, projRoot)
	if err != nil {
		return nil, "", "", err
	}
	if e, slash, err := resolveSkillFromBindings(binds, rest); err == nil {
		return e, slash, "", nil
	}
	bestIdx := -1
	bestLen := -1
	bestRemainder := ""
	restLower := strings.ToLower(rest)
	for i := range binds {
		name := strings.TrimSpace(binds[i].Entry.Name)
		if name == "" {
			continue
		}
		nameLower := strings.ToLower(name)
		if !strings.HasPrefix(restLower, nameLower) {
			continue
		}
		if len(restLower) > len(nameLower) {
			r, _ := utf8.DecodeRuneInString(rest[len(name):])
			if !unicode.IsSpace(r) {
				continue
			}
		}
		remainder := strings.TrimSpace(rest[len(name):])
		if len(name) > bestLen {
			bestIdx = i
			bestLen = len(name)
			bestRemainder = remainder
		}
	}
	if bestIdx >= 0 {
		e := binds[bestIdx].Entry
		return &e, binds[bestIdx].Slash, bestRemainder, nil
	}
	return nil, "", "", fmt.Errorf("skill not found: %q (try /skills)", rest)
}
