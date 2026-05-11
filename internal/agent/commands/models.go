package commands

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	readline "github.com/chzyer/readline"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/modelsapi"
)

type ListedModel struct {
	Prov  string
	Model string
}

type pickerRow struct {
	lm     ListedModel
	recent bool
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

func pickRecentListed(d Deps, all []ListedModel, cur ListedModel, max int) []ListedModel {
	if max <= 0 || d.Cfg == nil {
		return nil
	}
	claimed := map[string]bool{lmKey(cur): true}
	var out []ListedModel
	for _, u := range d.Cfg.RecentModelUses {
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

func assemblePickerRows(cur ListedModel, recents, prov []ListedModel) []pickerRow {
	out := []pickerRow{{lm: cur}}
	for i := range recents {
		out = append(out, pickerRow{lm: recents[i], recent: true})
	}
	for i := range prov {
		out = append(out, pickerRow{lm: prov[i], recent: false})
	}
	return out
}

func buildSlashPicker(d Deps, catalog []ListedModel) ([]pickerRow, map[string]bool) {
	cur := ListedModel{Prov: d.Provider().Name, Model: d.Model()}
	recents := pickRecentListed(d, catalog, cur, 5)
	claimed := map[string]bool{lmKey(cur): true}
	for i := range recents {
		claimed[lmKey(recents[i])] = true
	}
	need := 20 - len(recents)
	prov := fillProviderPicks(catalog, claimed, need)
	shown := map[string]bool{}
	for k, v := range claimed {
		shown[k] = v
	}
	return assemblePickerRows(cur, recents, prov), shown
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
	if d.ReadLine != nil {
		if s := strings.TrimSpace(bannerOneLine); s != "" {
			fmt.Fprintln(d.Out, s)
		}
		line, err := d.ReadLine(p)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(line), nil
	}
	if s := strings.TrimSpace(bannerOneLine); s != "" {
		fmt.Fprintln(d.Out, s)
	}
	fmt.Fprint(d.Out, p)
	br := bufio.NewScanner(d.Stdin)
	if !br.Scan() {
		if err := br.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return strings.TrimSpace(br.Text()), nil
}

func SlashModels(d Deps) error {
	var catalog []ListedModel
	for i := range d.Cfg.Providers {
		p := &d.Cfg.Providers[i]
		ids, err := modelsapi.List(p.BaseURL, p.APIKey)
		if err != nil {
			fmt.Fprintf(d.Out, "provider %s: error: %v\n", p.Name, err)
			continue
		}
		for _, mid := range ids {
			catalog = append(catalog, ListedModel{p.Name, mid})
		}
	}
	if len(catalog) == 0 {
		return fmt.Errorf("no models available")
	}
	pickerRows, shownKeys := buildSlashPicker(d, catalog)
	printSlashModelPicker(d.Out, pickerRows)
	hasUnlisted := false
	for i := range catalog {
		if !shownKeys[lmKey(catalog[i])] {
			hasUnlisted = true
			break
		}
	}
	if hasUnlisted {
		fmt.Fprintln(d.Out, "...")
	}
	for {
		banner := fmt.Sprintf("Select: index 0-%d", len(pickerRows)-1)
		banner += ", or paste exact model id"
		line, err := readInputLine(d, banner, "> ")
		if err != nil {
			if errors.Is(err, readline.ErrInterrupt) {
				fmt.Fprintln(d.Out, "^C")
			}
			return err
		}
		if line == "" {
			fmt.Fprintln(d.Out, "Invalid: empty input.")
			continue
		}
		ok, msg, ferr := trySlashModelPick(d, pickerRows, catalog, line)
		if ferr != nil {
			if errors.Is(ferr, readline.ErrInterrupt) {
				fmt.Fprintln(d.Out, "^C")
			}
			return ferr
		}
		if ok {
			fmt.Fprintf(d.Out, "Using %s[%s]\n", d.Model(), d.Provider().Name)
			return nil
		}
		fmt.Fprintf(d.Out, "Invalid: %s\n", msg)
	}
}

func printSlashModelPicker(out io.Writer, rows []pickerRow) {
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(out, "0\t%s[%s]\t(current)\n", rows[0].lm.Model, rows[0].lm.Prov)
	nRec := 0
	for i := 1; i < len(rows); i++ {
		if rows[i].recent {
			nRec++
		} else {
			break
		}
	}
	if nRec > 0 {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "[recents]")
		for i := 1; i <= nRec; i++ {
			pr := rows[i]
			fmt.Fprintf(out, "%d\t%s[%s]\t(recent)\n", i, pr.lm.Model, pr.lm.Prov)
		}
	}
	if nRec+1 < len(rows) {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "[models]")
		for i := nRec + 1; i < len(rows); i++ {
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
