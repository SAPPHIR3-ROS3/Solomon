package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	readline "github.com/chzyer/readline"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"
)

type ListedModel struct {
	Prov  string
	Model string
}

type pickerSection int

const (
	sectionCurrent pickerSection = iota
	sectionRecent
	sectionChatGPTSub
	sectionClaudeSub
	sectionCatalog
)

type pickerRow struct {
	lm      ListedModel
	section pickerSection
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

func pickChatGPTSubListed(d Deps, all []ListedModel, cur ListedModel, claimed map[string]bool, max int) []ListedModel {
	if max <= 0 || d.Cfg == nil {
		return nil
	}
	var out []ListedModel
	for _, u := range config.RecentModelUseEntries(d.Cfg, cur.Prov) {
		lm := ListedModel{Prov: strings.TrimSpace(u.Provider), Model: strings.TrimSpace(u.Model)}
		if lm.Prov != config.ProviderNameChatGPTSub || lm.Model == "" {
			continue
		}
		if !config.ModelPassesChatGPTSubFilter(lm.Model) {
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

func assemblePickerRows(cur ListedModel, recents, chatgptSub, claudeSub, prov []ListedModel) []pickerRow {
	out := []pickerRow{{lm: cur, section: sectionCurrent}}
	for i := range recents {
		out = append(out, pickerRow{lm: recents[i], section: sectionRecent})
	}
	for i := range chatgptSub {
		out = append(out, pickerRow{lm: chatgptSub[i], section: sectionChatGPTSub})
	}
	for i := range claudeSub {
		out = append(out, pickerRow{lm: claudeSub[i], section: sectionClaudeSub})
	}
	for i := range prov {
		out = append(out, pickerRow{lm: prov[i], section: sectionCatalog})
	}
	return out
}

func buildSlashPicker(d Deps, catalog []ListedModel) ([]pickerRow, map[string]bool) {
	cur := ListedModel{Prov: d.Provider().Name, Model: d.Model()}
	claimed := map[string]bool{lmKey(cur): true}
	recents := pickRecentListed(d, catalog, cur, claimed, 5)
	chatgptSub := pickChatGPTSubListed(d, catalog, cur, claimed, 5)
	claudeSub := pickClaudeSubListed(d, catalog, cur, claimed, 5)
	need := 20 - len(recents) - len(chatgptSub) - len(claudeSub)
	prov := fillProviderPicks(catalog, claimed, need)
	shown := map[string]bool{}
	for k, v := range claimed {
		shown[k] = v
	}
	return assemblePickerRows(cur, recents, chatgptSub, claudeSub, prov), shown
}

func fillProviderPicks(catalog []ListedModel, claimed map[string]bool, need int) []ListedModel {
	if need <= 0 {
		return nil
	}
	var out []ListedModel
	for i := range catalog {
		lm := catalog[i]
		if claimed[lmKey(lm)] {
			continue
		}
		out = append(out, lm)
		claimed[lmKey(lm)] = true
		if len(out) >= need {
			break
		}
	}
	return out
}

func readInputLine(d Deps, bannerOneLine string, rlPrompt string) (string, error) {
	p := rlPrompt
	if p == "" {
		p = "> "
	}
	if s := strings.TrimSpace(bannerOneLine); s != "" {
		PrintSystem(d.Out, s)
	}
	return config.ReadPromptLine(PromptIO(d), p)
}

func SlashModels(d Deps) error {
	ctx := d.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	var catalog []ListedModel
	for _, p := range config.ProviderList(d.Cfg) {
		pp := p
		ids, err := listModelsForProvider(ctx, d.Cfg, &pp)
		if err != nil {
			PrintSystemf(d.Out, "provider %s: error: %v", p.Name, err)
			continue
		}
		for _, mid := range ids {
			catalog = append(catalog, ListedModel{pp.Name, mid})
		}
	}
	if len(catalog) == 0 {
		return fmt.Errorf("no models available")
	}
	pickerRows, shownKeys := buildSlashPicker(d, catalog)
	var pickerBuf bytes.Buffer
	printSlashModelPicker(&pickerBuf, pickerRows)
	termcolor.WriteSystem(d.Out, pickerBuf.String())
	hasUnlisted := false
	for i := range catalog {
		if !shownKeys[lmKey(catalog[i])] {
			hasUnlisted = true
			break
		}
	}
	if hasUnlisted {
		PrintSystem(d.Out, "...")
	}
	for {
		banner := fmt.Sprintf("Select: index 0-%d", len(pickerRows)-1)
		banner += ", or paste exact model id"
		line, err := readInputLine(d, banner, "> ")
		if err != nil {
			if errors.Is(err, readline.ErrInterrupt) {
				PrintSystem(d.Out, "^C")
			}
			return err
		}
		if line == "" {
			PrintSystem(d.Out, "Invalid: empty input.")
			continue
		}
		ok, msg, ferr := trySlashModelPick(d, pickerRows, catalog, line)
		if ferr != nil {
			if errors.Is(ferr, readline.ErrInterrupt) {
				PrintSystem(d.Out, "^C")
			}
			return ferr
		}
		if ok {
			PrintSystemf(d.Out, "Using %s[%s]", d.Model(), d.Provider().Name)
			return nil
		}
		PrintSystemf(d.Out, "Invalid: %s", msg)
	}
}

func printSlashModelPicker(out io.Writer, rows []pickerRow) {
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(out, "0\t%s[%s]\t(current)\n", rows[0].lm.Model, rows[0].lm.Prov)
	i := 1
	printSection := func(title string, sec pickerSection, tag string) {
		if i >= len(rows) || rows[i].section != sec {
			return
		}
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, title)
		for i < len(rows) && rows[i].section == sec {
			pr := rows[i]
			fmt.Fprintf(out, "%d\t%s[%s]\t%s\n", i, pr.lm.Model, pr.lm.Prov, tag)
			i++
		}
	}
	printSection("[recents]", sectionRecent, "(recent)")
	printSection("[ChatGPT Sub]", sectionChatGPTSub, "(ChatGPT Sub)")
	printSection("[Claude Sub]", sectionClaudeSub, "(Claude Sub)")
	if i < len(rows) {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "[models]")
		for ; i < len(rows); i++ {
			pr := rows[i]
			fmt.Fprintf(out, "%d\t%s[%s]\n", i, pr.lm.Model, pr.lm.Prov)
		}
	}
}

func trySlashModelPick(d Deps, picker []pickerRow, catalog []ListedModel, line string) (ok bool, errMsg string, err error) {
	if config.AllDigits(line) {
		n, ierr := strconv.Atoi(line)
		if ierr != nil {
			return false, "not a valid number.", nil
		}
		if n < 0 || n >= len(picker) {
			return false, fmt.Sprintf("index must be between 0 and %d.", len(picker)-1), nil
		}
		pr := picker[n]
		if aerr := d.ApplyCurrentModel(pr.lm.Prov, pr.lm.Model); aerr != nil {
			return false, aerr.Error(), nil
		}
		return true, "", nil
	}
	if rerr := resolveModelPaste(d, catalog, line); rerr != nil {
		return false, rerr.Error(), nil
	}
	return true, "", nil
}

func resolveModelPaste(d Deps, rows []ListedModel, id string) error {
	var matches []ListedModel
	for _, row := range rows {
		if row.Model == id {
			matches = append(matches, row)
		}
	}
	if len(matches) == 0 {
		return fmt.Errorf("model id %q not in the listed models", id)
	}
	if len(matches) > 1 {
		return fmt.Errorf("model id %q exists for multiple providers; use the numeric index from /models", id)
	}
	return d.ApplyCurrentModel(matches[0].Prov, matches[0].Model)
}
