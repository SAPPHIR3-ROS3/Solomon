package commands

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	cursorint "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

type ListedModel struct {
	Prov  string
	Model string
}

type pickerSection int

const (
	sectionCurrent pickerSection = iota
	sectionRecent
	sectionClaudeSub
	sectionCatalog
)

type pickerRow struct {
	lm      ListedModel
	section pickerSection
	lineTag string
}

func (pr pickerRow) displayTag() string {
	if pr.lineTag != "" {
		return pr.lineTag
	}
	return pickerSectionTag[pr.section]
}

func lmKey(l ListedModel) string {
	return l.Prov + "\x00" + l.Model
}

func catalogContains(all []ListedModel, lm ListedModel) bool {
	k := lmKey(lm)
	for i := range all {
		if lmKey(all[i]) == k {
			return true
		}
	}
	return false
}

func fillProviderCatalogBatch(catalog []ListedModel, skip map[string]bool, need int, prov string) []ListedModel {
	if need <= 0 {
		return nil
	}
	var out []ListedModel
	for i := range catalog {
		lm := catalog[i]
		if lm.Prov != prov {
			continue
		}
		if skip[lmKey(lm)] {
			continue
		}
		out = append(out, lm)
		skip[lmKey(lm)] = true
		if len(out) >= need {
			break
		}
	}
	return out
}

func pickRecentListedForProvider(d Deps, all []ListedModel, cur ListedModel, claimed map[string]bool, max int, prov string) []ListedModel {
	if max <= 0 || d.Cfg == nil {
		return nil
	}
	var out []ListedModel
	for _, u := range config.RecentModelUseEntries(d.Cfg, prov) {
		lm := ListedModel{Prov: strings.TrimSpace(u.Provider), Model: strings.TrimSpace(u.Model)}
		if lm.Prov != prov || lm.Model == "" {
			continue
		}
		if lmKey(lm) == lmKey(cur) {
			continue
		}
		if claimed[lmKey(lm)] {
			continue
		}
		if !catalogContains(all, lm) {
			continue
		}
		out = append(out, lm)
		claimed[lmKey(lm)] = true
		if len(out) >= max {
			break
		}
	}
	return out
}

func pickRecentListed(d Deps, all []ListedModel, cur ListedModel, claimed map[string]bool, max int) []ListedModel {
	if max <= 0 || d.Cfg == nil {
		return nil
	}
	var out []ListedModel
	for _, u := range config.RecentModelUseEntries(d.Cfg, cur.Prov) {
		lm := ListedModel{Prov: strings.TrimSpace(u.Provider), Model: strings.TrimSpace(u.Model)}
		if lm.Prov == "" || lm.Model == "" {
			continue
		}
		if lmKey(lm) == lmKey(cur) {
			continue
		}
		if claimed[lmKey(lm)] {
			continue
		}
		if !catalogContains(all, lm) {
			continue
		}
		out = append(out, lm)
		claimed[lmKey(lm)] = true
		if len(out) >= max {
			break
		}
	}
	return out
}

func claimOtherChatGPTSubCatalog(catalog []ListedModel, cur ListedModel, claimed map[string]bool) {
	for i := range catalog {
		lm := catalog[i]
		if lm.Prov != config.ProviderNameChatGPTSub {
			continue
		}
		if lmKey(lm) == lmKey(cur) {
			continue
		}
		if claimed[lmKey(lm)] {
			continue
		}
		claimed[lmKey(lm)] = true
	}
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

func gptFamilyKeyFromModel(model string) ([]int, bool) {
	if !config.ModelPassesChatGPTSubPickerFilter(model) {
		return nil, false
	}
	m := strings.ToLower(strings.TrimSpace(model))
	rest := strings.TrimPrefix(m, "gpt-")
	parts := strings.Split(rest, "-")
	key := parseVersionSegment(parts[0])
	if len(key) == 0 {
		return nil, false
	}
	return key, true
}

func gptVariantSuffix(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	rest := strings.TrimPrefix(m, "gpt-")
	parts := strings.Split(rest, "-")
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[1:], "-")
}

func preferredGPTVariants(family []int) []string {
	switch {
	case len(family) == 2 && family[0] == 5 && family[1] == 4:
		return []string{"", "mini"}
	case len(family) == 2 && family[0] == 5 && family[1] == 3:
		return []string{"codex", ""}
	case len(family) == 2 && family[0] == 5 && family[1] == 2:
		return []string{""}
	case len(family) == 1 && family[0] == 5:
		return []string{""}
	default:
		return []string{"", "codex", "mini", "nano"}
	}
}

func gptVariantPreferenceScore(family []int, variant string) int {
	prefs := preferredGPTVariants(family)
	for i, v := range prefs {
		if v == variant {
			return len(prefs) - i
		}
	}
	return -1
}

func compareGPTFamilies(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
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

func familyKeyString(family []int) string {
	parts := make([]string, len(family))
	for i, n := range family {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ".")
}

func bestChatGPTSubForFamily(family []int, cands []ListedModel) (ListedModel, bool) {
	bestScore := -1
	var chosen ListedModel
	found := false
	for i := range cands {
		score := gptVariantPreferenceScore(family, gptVariantSuffix(cands[i].Model))
		if score < 0 {
			continue
		}
		if score > bestScore {
			bestScore = score
			chosen = cands[i]
			found = true
		}
	}
	return chosen, found
}

func pickChatGPTSubFamilySlots(all []ListedModel, cur ListedModel, skip map[string]bool, max int) []ListedModel {
	if max <= 0 {
		return nil
	}
	byFamily := map[string][]ListedModel{}
	var families [][]int
	for i := range all {
		lm := all[i]
		if lm.Prov != config.ProviderNameChatGPTSub || lm.Model == "" {
			continue
		}
		if lmKey(lm) == lmKey(cur) {
			continue
		}
		if skip != nil && skip[lmKey(lm)] {
			continue
		}
		family, ok := gptFamilyKeyFromModel(lm.Model)
		if !ok {
			continue
		}
		key := familyKeyString(family)
		if _, exists := byFamily[key]; !exists {
			families = append(families, family)
		}
		byFamily[key] = append(byFamily[key], lm)
	}
	sort.Slice(families, func(i, j int) bool {
		return compareGPTFamilies(families[i], families[j]) > 0
	})
	var out []ListedModel
	for _, family := range families {
		if len(out) >= max {
			break
		}
		lm, ok := bestChatGPTSubForFamily(family, byFamily[familyKeyString(family)])
		if ok {
			out = append(out, lm)
		}
	}
	return out
}

func pickClaudeSubListed(d Deps, all []ListedModel, cur ListedModel, claimed map[string]bool, max int) []ListedModel {
	if max <= 0 || d.Cfg == nil {
		return nil
	}
	var out []ListedModel
	for _, u := range config.RecentModelUseEntries(d.Cfg, cur.Prov) {
		lm := ListedModel{Prov: strings.TrimSpace(u.Provider), Model: strings.TrimSpace(u.Model)}
		if lm.Prov != config.ProviderNameClaudeSub || lm.Model == "" {
			continue
		}
		if !config.ModelPassesClaudeSubFilter(lm.Model) {
			continue
		}
		if lmKey(lm) == lmKey(cur) {
			continue
		}
		if claimed[lmKey(lm)] {
			continue
		}
		if !catalogContains(all, lm) {
			continue
		}
		out = append(out, lm)
		claimed[lmKey(lm)] = true
		if len(out) >= max {
			break
		}
	}
	return out
}

func orderListedModelsByProvider(lms []ListedModel, prov string) []ListedModel {
	if len(lms) == 0 {
		return lms
	}
	switch prov {
	case config.ProviderNameChatGPTSub:
		return orderChatGPTSubListedModels(lms)
	case config.ProviderNameCursorAPI:
		ids := make([]string, len(lms))
		for i := range lms {
			ids[i] = lms[i].Model
		}
		return orderListedModelsFromIDs(prov, cursorint.OrderModelIDs(ids))
	default:
		return lms
	}
}

func orderListedModelsFromIDs(prov string, ids []string) []ListedModel {
	out := make([]ListedModel, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out = append(out, ListedModel{Prov: prov, Model: id})
	}
	return out
}

func orderChatGPTSubListedModels(lms []ListedModel) []ListedModel {
	var filtered []ListedModel
	for i := range lms {
		if config.ModelPassesChatGPTSubFilter(lms[i].Model) {
			filtered = append(filtered, lms[i])
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		fi, oki := gptFamilyKeyFromModel(filtered[i].Model)
		fj, okj := gptFamilyKeyFromModel(filtered[j].Model)
		if oki && okj {
			if c := compareGPTFamilies(fi, fj); c != 0 {
				return c > 0
			}
			si := gptVariantPreferenceScore(fi, gptVariantSuffix(filtered[i].Model))
			sj := gptVariantPreferenceScore(fj, gptVariantSuffix(filtered[j].Model))
			if si != sj {
				return si > sj
			}
		} else if oki != okj {
			return oki
		}
		return filtered[i].Model < filtered[j].Model
	})
	return filtered
}

func assemblePickerRows(cur ListedModel, recents, claudeSub, catalogRows []ListedModel) []pickerRow {
	out := []pickerRow{{lm: cur, section: sectionCurrent}}
	for i := range recents {
		out = append(out, pickerRow{lm: recents[i], section: sectionRecent})
	}
	for i := range claudeSub {
		out = append(out, pickerRow{lm: claudeSub[i], section: sectionClaudeSub})
	}
	for i := range catalogRows {
		out = append(out, pickerRow{lm: catalogRows[i], section: sectionCatalog})
	}
	return out
}

func pickerIndexColWidth(rowCount int) int {
	if rowCount <= 0 {
		return 5
	}
	digits := len(strconv.Itoa(rowCount - 1))
	if digits < 1 {
		digits = 1
	}
	return digits + 3
}

func pickerMaxModelLen(rows []pickerRow) int {
	max := 0
	for i := range rows {
		if n := len(rows[i].lm.Model); n > max {
			max = n
		}
	}
	return max
}

func pickerMaxProvBracketLen(rows []pickerRow) int {
	max := 0
	for i := range rows {
		n := len(rows[i].lm.Prov) + 2
		if n > max {
			max = n
		}
	}
	return max
}

var pickerSectionTag = map[pickerSection]string{
	sectionCurrent:   "(current)",
	sectionRecent:    "(recent)",
	sectionClaudeSub: "(Claude Sub)",
}

func pickerMaxTagLen(rows []pickerRow) int {
	max := 0
	for i := range rows {
		if n := len(rows[i].displayTag()); n > max {
			max = n
		}
	}
	return max
}

func writePickerModelLine(out io.Writer, idx, idxColW, modelColW, provColW, tagColW int, model, prov, tag string) {
	provBracket := fmt.Sprintf("[%s]", prov)
	if tag == "" {
		fmt.Fprintf(out, "%-*d%-*s\t%-*s\n", idxColW, idx, modelColW, model, provColW, provBracket)
		return
	}
	fmt.Fprintf(out, "%-*d%-*s\t%-*s\t%-*s\n", idxColW, idx, modelColW, model, provColW, provBracket, tagColW, tag)
}

