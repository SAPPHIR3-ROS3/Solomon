package commands

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"solomon/internal/config"
	"solomon/internal/modelsapi"
)

type ListedModel struct {
	Prov  string
	Model string
}

func SlashModels(d Deps) error {
	var rows []ListedModel
	for i := range d.Cfg.Providers {
		p := &d.Cfg.Providers[i]
		ids, err := modelsapi.List(p.BaseURL, p.APIKey)
		if err != nil {
			fmt.Fprintf(d.Out, "provider %s: error: %v\n", p.Name, err)
			continue
		}
		for _, mid := range ids {
			rows = append(rows, ListedModel{p.Name, mid})
		}
	}
	if len(rows) == 0 {
		return fmt.Errorf("no models available")
	}
	stdin := d.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	nShow := len(rows)
	if len(rows) > 20 {
		nShow = 20
	}
	for i := 0; i < nShow; i++ {
		mark := ""
		if d.Cfg.Current.Provider == rows[i].Prov && d.Cfg.Current.Model == rows[i].Model {
			mark = "\t(current)"
		}
		fmt.Fprintf(d.Out, "%d\t%s[%s]%s\n", i, rows[i].Model, rows[i].Prov, mark)
	}
	if len(rows) > 20 {
		fmt.Fprintln(d.Out, "...")
	}
	br := bufio.NewScanner(stdin)
	for {
		fmt.Fprintf(d.Out, "Select: index 0-%d", nShow-1)
		if len(rows) > 20 {
			fmt.Fprint(d.Out, ", 20 to enter a model id, or paste exact model id")
		} else {
			fmt.Fprint(d.Out, ", or paste exact model id")
		}
		fmt.Fprint(d.Out, "\n> ")
		br.Scan()
		line := strings.TrimSpace(br.Text())
		if line == "" {
			fmt.Fprintln(d.Out, "Invalid: empty input.")
			continue
		}
		if len(rows) > 20 {
			ok, msg := trySlashModelPick(d, rows, line, br)
			if ok {
				fmt.Fprintf(d.Out, "Using %s[%s]\n", d.Model(), d.Provider().Name)
				return nil
			}
			fmt.Fprintf(d.Out, "Invalid: %s\n", msg)
			continue
		}
		if config.AllDigits(line) {
			n, err := strconv.Atoi(line)
			if err != nil {
				fmt.Fprintln(d.Out, "Invalid: not a valid number.")
				continue
			}
			if n < 0 || n >= len(rows) {
				fmt.Fprintf(d.Out, "Invalid: index must be between 0 and %d.\n", len(rows)-1)
				continue
			}
			sel := rows[n]
			if err := d.ApplyCurrentModel(sel.Prov, sel.Model); err != nil {
				return err
			}
			fmt.Fprintf(d.Out, "Using %s[%s]\n", d.Model(), d.Provider().Name)
			return nil
		}
		if err := resolveModelPaste(d, rows, line); err != nil {
			fmt.Fprintf(d.Out, "Invalid: %v\n", err)
			continue
		}
		fmt.Fprintf(d.Out, "Using %s[%s]\n", d.Model(), d.Provider().Name)
		return nil
	}
}

func trySlashModelPick(d Deps, rows []ListedModel, line string, br *bufio.Scanner) (ok bool, errMsg string) {
	if config.AllDigits(line) {
		n, err := strconv.Atoi(line)
		if err != nil {
			return false, "not a valid number."
		}
		if n >= 0 && n < 20 {
			sel := rows[n]
			if err := d.ApplyCurrentModel(sel.Prov, sel.Model); err != nil {
				return false, err.Error()
			}
			return true, ""
		}
		if n == 20 {
			for {
				fmt.Fprint(d.Out, "Model id: ")
				br.Scan()
				id := strings.TrimSpace(br.Text())
				if id == "" {
					fmt.Fprintln(d.Out, "Invalid: empty model id.")
					continue
				}
				if err := resolveModelPaste(d, rows, id); err != nil {
					fmt.Fprintf(d.Out, "Invalid: %v\n", err)
					continue
				}
				return true, ""
			}
		}
		return false, "index must be 0-19 or 20 to type an id."
	}
	if err := resolveModelPaste(d, rows, line); err != nil {
		return false, err.Error()
	}
	return true, ""
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
		return fmt.Errorf("model id %q exists for multiple providers; use a numeric index 0-19", id)
	}
	return d.ApplyCurrentModel(matches[0].Prov, matches[0].Model)
}
