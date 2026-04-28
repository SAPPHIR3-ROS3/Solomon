package commands

import (
	"fmt"
	"strconv"
)

func MaxResponse(d Deps, parts []string) error {
	if len(parts) < 2 {
		if d.Cfg.MaxResponseTokens > 0 {
			fmt.Fprintf(d.Out, "max_response_tokens=%d (max_completion_tokens)\n", d.Cfg.MaxResponseTokens)
		} else {
			fmt.Fprintln(d.Out, "max_response_tokens unset (provider/model default)")
		}
		return nil
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}
	if n < 1 {
		return fmt.Errorf("max_response must be >= 1")
	}
	d.Cfg.MaxResponseTokens = n
	if err := d.SaveCfg(); err != nil {
		return err
	}
	fmt.Fprintf(d.Out, "max_response_tokens=%d\n", n)
	return nil
}
