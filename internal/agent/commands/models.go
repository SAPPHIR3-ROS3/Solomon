package commands

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	cursorint "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelsapi"
)

type ListedModel struct {
	Prov  string
	Model string
}

type pickerSection int

const (
	sectionCurrent pickerSection = iota
	sectionRecent
	sectionProvider
	sectionCatalog
)

type pickerRow struct {
	lm       ListedModel
	section  pickerSection
	lineTag  string
	provOnly string
}

func (pr pickerRow) displayTag() string {
	if pr.lineTag != "" {
		return pr.lineTag
	}
	return pickerSectionTag[pr.section]
}

func (pr pickerRow) isProvider() bool {
	return strings.TrimSpace(pr.provOnly) != ""
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

func orderListedModelsByProvider(lms []ListedModel, prov string) []ListedModel {
	if len(lms) == 0 {
		return lms
	}
	switch prov {
	case config.ProviderNameChatGPTSub:
		return orderChatGPTSubListedModels(lms)
	case config.ProviderNameClaudeSub:
		return orderClaudeSubListedModels(lms)
	case config.ProviderNameCursorAPI:
		ids := make([]string, len(lms))
		for i := range lms {
			ids[i] = lms[i].Model
		}
		return orderListedModelsFromIDs(prov, cursorint.OrderModelIDs(ids))
	default:
		return orderListedModelsAlphabetical(lms)
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

func orderListedModelsAlphabetical(lms []ListedModel) []ListedModel {
	out := append([]ListedModel(nil), lms...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Model < out[j].Model
	})
	return out
}

func modelVersionKey(model string) []int {
	m := strings.ToLower(strings.TrimSpace(model))
	var key []int
	i := 0
	for i < len(m) {
		if m[i] < '0' || m[i] > '9' {
			i++
			continue
		}
		j := i
		for j < len(m) && m[j] >= '0' && m[j] <= '9' {
			j++
		}
		n, _ := strconv.Atoi(m[i:j])
		key = append(key, n)
		i = j
	}
	return key
}

func compareVersionKeysDesc(a, b []int) int {
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

func modelHasSegment(model, keyword string) bool {
	for _, seg := range strings.Split(strings.ToLower(strings.TrimSpace(model)), "-") {
		if seg == keyword {
			return true
		}
	}
	return false
}

func isChatGPTBaseModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if !strings.HasPrefix(m, "gpt") {
		return false
	}
	for _, seg := range strings.Split(m, "-") {
		switch seg {
		case "sol", "terra", "luna", "codex", "mini", "nano", "pro":
			return false
		}
	}
	return true
}

func bestListedMatching(lms []ListedModel, claimed map[string]bool, match func(string) bool) (ListedModel, bool) {
	var best ListedModel
	var bestVer []int
	found := false
	for i := range lms {
		lm := lms[i]
		if claimed[lmKey(lm)] || !match(lm.Model) {
			continue
		}
		ver := modelVersionKey(lm.Model)
		if !found || compareVersionKeysDesc(ver, bestVer) > 0 || (compareVersionKeysDesc(ver, bestVer) == 0 && lm.Model < best.Model) {
			best = lm
			bestVer = ver
			found = true
		}
	}
	return best, found
}

func orderChatGPTSubListedModels(lms []ListedModel) []ListedModel {
	if len(lms) == 0 {
		return lms
	}
	claimed := map[string]bool{}
	var out []ListedModel
	pick := func(match func(string) bool) {
		lm, ok := bestListedMatching(lms, claimed, match)
		if !ok {
			return
		}
		out = append(out, lm)
		claimed[lmKey(lm)] = true
	}
	pick(func(m string) bool { return modelHasSegment(m, "sol") })
	pick(func(m string) bool { return modelHasSegment(m, "terra") })
	pick(func(m string) bool { return modelHasSegment(m, "luna") })
	pick(isChatGPTBaseModel)
	pick(func(m string) bool { return modelHasSegment(m, "codex") })
	pick(func(m string) bool { return modelHasSegment(m, "mini") })
	pick(func(m string) bool { return modelHasSegment(m, "nano") })
	var rest []ListedModel
	for i := range lms {
		if claimed[lmKey(lms[i])] {
			continue
		}
		rest = append(rest, lms[i])
	}
	sort.Slice(rest, func(i, j int) bool {
		if c := compareVersionKeysDesc(modelVersionKey(rest[i].Model), modelVersionKey(rest[j].Model)); c != 0 {
			return c > 0
		}
		return rest[i].Model < rest[j].Model
	})
	return append(out, rest...)
}

func orderClaudeSubListedModels(lms []ListedModel) []ListedModel {
	if len(lms) == 0 {
		return lms
	}
	ids := make([]string, len(lms))
	for i := range lms {
		ids[i] = lms[i].Model
	}
	ordered := modelsapi.OrderClaudeSubModelIDs(ids)
	out := make([]ListedModel, 0, len(lms))
	for _, id := range ordered {
		for i := range lms {
			if lms[i].Model == id {
				out = append(out, lms[i])
				break
			}
		}
	}
	return out
}

var pickerSectionTag = map[pickerSection]string{
	sectionCurrent:  "(current)",
	sectionRecent:   "(recent)",
	sectionProvider: "",
	sectionCatalog:  "",
}

func pickerMaxModelLen(rows []pickerRow) int {
	max := 0
	for i := range rows {
		label := rows[i].lm.Model
		if rows[i].isProvider() {
			label = rows[i].provOnly
		}
		if n := len(label); n > max {
			max = n
		}
	}
	return max
}

func pickerMaxProvBracketLen(rows []pickerRow) int {
	max := 0
	for i := range rows {
		if rows[i].isProvider() {
			continue
		}
		n := len(rows[i].lm.Prov) + 2
		if n > max {
			max = n
		}
	}
	return max
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
		fmt.Fprintf(out, "%*d  %-*s\t%-*s\n", idxColW, idx, modelColW, model, provColW, provBracket)
		return
	}
	fmt.Fprintf(out, "%*d  %-*s\t%-*s\t%-*s\n", idxColW, idx, modelColW, model, provColW, provBracket, tagColW, tag)
}

func writePickerProviderLine(out io.Writer, idx, idxColW, modelColW int, prov string) {
	fmt.Fprintf(out, "%*d  %-*s\n", idxColW, idx, modelColW, prov)
}

func writeSlashModelPickerHelp(out io.Writer, lastIdx int, hasMore bool, filterProv string) {
	var b strings.Builder
	fmt.Fprintf(&b, "Select: index 0-%d", lastIdx)
	if filterProv == "" {
		b.WriteString(", provider index/name to open models")
	} else {
		b.WriteString(", paste exact model id")
	}
	if hasMore {
		b.WriteString(", > for next page")
	}
	if filterProv != "" {
		fmt.Fprintf(&b, " (filtered: %s; type all to reset)", filterProv)
	}
	fmt.Fprintln(out, b.String())
}

func printSlashModelPickerDisplay(out io.Writer, display []slashPickerDisplayRow) {
	if len(display) == 0 {
		return
	}
	rows := make([]pickerRow, len(display))
	for i := range display {
		rows[i] = display[i].pr
	}
	idxColW := pickerIndexColWidthForMax(displayMaxIndex(display))
	modelColW := pickerMaxModelLen(rows)
	provColW := pickerMaxProvBracketLen(rows)
	tagColW := pickerMaxTagLen(rows)
	printedRecents, printedProviders, printedModels := false, false, false
	for _, dr := range display {
		switch dr.pr.section {
		case sectionRecent:
			if !printedRecents {
				fmt.Fprintln(out, "[recents]")
				printedRecents = true
			}
		case sectionProvider:
			if !printedProviders {
				fmt.Fprintln(out, "[providers]")
				printedProviders = true
			}
		case sectionCatalog:
			if !printedModels {
				fmt.Fprintln(out, "[models]")
				printedModels = true
			}
		}
		if dr.pr.isProvider() {
			writePickerProviderLine(out, dr.index, idxColW, modelColW, dr.pr.provOnly)
			continue
		}
		writePickerModelLine(out, dr.index, idxColW, modelColW, provColW, tagColW, dr.pr.lm.Model, dr.pr.lm.Prov, dr.pr.displayTag())
	}
}

func displayMaxIndex(display []slashPickerDisplayRow) int {
	max := 0
	for _, dr := range display {
		if dr.index > max {
			max = dr.index
		}
	}
	return max
}

func pickerIndexColWidthForMax(maxIdx int) int {
	digits := len(strconv.Itoa(maxIdx))
	if digits < 1 {
		digits = 1
	}
	return digits
}
