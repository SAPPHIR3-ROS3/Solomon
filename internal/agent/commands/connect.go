package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"solomon/internal/config"
)

func Connect(d Deps) error {
	stdin := d.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	br := bufio.NewReader(stdin)
	fmt.Fprint(d.Out, "Provider display name: ")
	n, _ := br.ReadString('\n')
	fmt.Fprint(d.Out, "Base URL: ")
	u, _ := br.ReadString('\n')
	fmt.Fprint(d.Out, "API key: ")
	k, _ := br.ReadString('\n')
	base, err := config.NormalizeAPIBase(strings.TrimSpace(u))
	if err != nil {
		return err
	}
	prov := config.Provider{Name: strings.TrimSpace(n), BaseURL: base, APIKey: strings.TrimSpace(k)}
	d.Cfg.Providers = append(d.Cfg.Providers, prov)
	d.Cfg.Current.Provider = prov.Name
	return d.SaveCfg()
}

