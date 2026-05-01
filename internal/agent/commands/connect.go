package commands

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/modelsapi"
)

func Connect(d Deps) error {
	stdin := d.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	sc := bufio.NewScanner(stdin)
	fmt.Fprint(d.Out, "Provider display name: ")
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return err
		}
		return fmt.Errorf("missing provider display name")
	}
	n := strings.TrimSpace(sc.Text())
	fmt.Fprint(d.Out, "Base URL: ")
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return err
		}
		return fmt.Errorf("missing base URL")
	}
	u := strings.TrimSpace(sc.Text())
	fmt.Fprint(d.Out, "API key: ")
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return err
		}
		return fmt.Errorf("missing API key")
	}
	k := strings.TrimSpace(sc.Text())
	base, err := config.NormalizeAPIBase(u)
	if err != nil {
		return err
	}
	ids, err := modelsapi.List(base, k)
	if err != nil {
		return fmt.Errorf("connection check failed: %w", err)
	}
	prevProv := d.Cfg.Current.Provider
	prevModel := d.Cfg.Current.Model
	prov := config.Provider{Name: n, BaseURL: base, APIKey: k}
	d.Cfg.Providers = append(d.Cfg.Providers, prov)
	if err := d.SaveCfg(); err != nil {
		d.Cfg.Providers = d.Cfg.Providers[:len(d.Cfg.Providers)-1]
		return err
	}
	return connectPickModel(d, sc, prevProv, prevModel, prov.Name, ids)
}

func connectPickModel(d Deps, br *bufio.Scanner, prevProv, prevModel, newProvName string, newIDs []string) error {
	const maxShown = 20
	nShownNew := len(newIDs)
	truncated := false
	if nShownNew > maxShown {
		nShownNew = maxShown
		truncated = true
	}
	fmt.Fprintf(d.Out, "0\t%s\t[%s]\t(current)\n", prevModel, prevProv)
	for i := 0; i < nShownNew; i++ {
		fmt.Fprintf(d.Out, "%d\t%s\t[%s]\n", i+1, newIDs[i], newProvName)
	}
	if truncated {
		fmt.Fprintln(d.Out, "...")
	}
	pasteIdx := 21
	for {
		fmt.Fprintf(d.Out, "Select: 0 = keep current provider/model")
		if nShownNew > 0 {
			fmt.Fprintf(d.Out, ", 1-%d = model on %s", nShownNew, newProvName)
		}
		if truncated {
			fmt.Fprintf(d.Out, ", %d = enter model id", pasteIdx)
		}
		fmt.Fprint(d.Out, ", or paste exact model id for the new provider\n> ")
		if !br.Scan() {
			if err := br.Err(); err != nil {
				return err
			}
			fmt.Fprintln(d.Out, "Invalid: empty input.")
			continue
		}
		line := strings.TrimSpace(br.Text())
		if line == "" {
			fmt.Fprintln(d.Out, "Invalid: empty input.")
			continue
		}
		ok, handled, cerr := connectTryPick(d, br, line, prevProv, prevModel, newProvName, newIDs, nShownNew, truncated, pasteIdx)
		if cerr != nil {
			return cerr
		}
		if handled && ok {
			fmt.Fprintf(d.Out, "Using %s[%s]\n", d.Model(), d.Provider().Name)
			return nil
		}
		if handled && !ok {
			continue
		}
		if err := connectResolvePasteNewProvider(d, newProvName, newIDs, line); err != nil {
			fmt.Fprintf(d.Out, "Invalid: %v\n", err)
			continue
		}
		fmt.Fprintf(d.Out, "Using %s[%s]\n", d.Model(), d.Provider().Name)
		return nil
	}
}

func connectTryPick(d Deps, br *bufio.Scanner, line, prevProv, prevModel, newProvName string, newIDs []string, nShownNew int, truncated bool, pasteIdx int) (ok bool, handled bool, err error) {
	if !config.AllDigits(line) {
		return false, false, nil
	}
	n, atoiErr := strconv.Atoi(line)
	if atoiErr != nil {
		return false, false, nil
	}
	if n == 0 {
		if err := d.ApplyCurrentModel(prevProv, prevModel); err != nil {
			return false, true, err
		}
		return true, true, nil
	}
	if n >= 1 && n <= nShownNew {
		if err := d.ApplyCurrentModel(newProvName, newIDs[n-1]); err != nil {
			return false, true, err
		}
		return true, true, nil
	}
	if truncated && n == pasteIdx {
		for {
			fmt.Fprint(d.Out, "Model id: ")
			if !br.Scan() {
				if err := br.Err(); err != nil {
					return false, true, err
				}
				fmt.Fprintln(d.Out, "Invalid: empty input.")
				continue
			}
			id := strings.TrimSpace(br.Text())
			if id == "" {
				fmt.Fprintln(d.Out, "Invalid: empty model id.")
				continue
			}
			if err := connectResolvePasteNewProvider(d, newProvName, newIDs, id); err != nil {
				fmt.Fprintf(d.Out, "Invalid: %v\n", err)
				continue
			}
			return true, true, nil
		}
	}
	fmt.Fprintf(d.Out, "Invalid: choose 0")
	if nShownNew > 0 {
		fmt.Fprintf(d.Out, ", 1-%d", nShownNew)
	}
	if truncated {
		fmt.Fprintf(d.Out, ", or %d for model id", pasteIdx)
	}
	fmt.Fprintf(d.Out, ", or paste a model id.\n")
	return false, true, nil
}

func connectResolvePasteNewProvider(d Deps, newProvName string, newIDs []string, id string) error {
	if len(newIDs) == 0 {
		return d.ApplyCurrentModel(newProvName, id)
	}
	for _, mid := range newIDs {
		if mid == id {
			return d.ApplyCurrentModel(newProvName, id)
		}
	}
	return fmt.Errorf("model id %q not in models returned by this provider", id)
}
