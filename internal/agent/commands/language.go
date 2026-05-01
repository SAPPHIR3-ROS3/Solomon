package commands

import (
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
)

func Language(d Deps, parts []string) error {
	if len(parts) < 2 {
		stored := strings.TrimSpace(d.Cfg.ResponseLanguage)
		eff := d.Cfg.EffectiveResponseLanguage()
		if stored == "" {
			fmt.Fprintf(d.Out, "response_language=%s (default)\n", eff)
		} else {
			fmt.Fprintf(d.Out, "response_language=%s\n", eff)
		}
		return nil
	}
	rest := strings.Join(parts[1:], " ")
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return fmt.Errorf("usage: /language | /language <language> | /language clear")
	}
	switch strings.ToLower(rest) {
	case "clear", "default", "reset":
		d.Cfg.ResponseLanguage = ""
	default:
		d.Cfg.ResponseLanguage = rest
	}
	if err := d.SaveCfg(); err != nil {
		return err
	}
	if strings.TrimSpace(d.Cfg.ResponseLanguage) != "" {
		fmt.Fprintf(d.Out, "response_language=%s (saved; injected into system prompt)\n", strings.TrimSpace(d.Cfg.ResponseLanguage))
	} else {
		fmt.Fprintf(d.Out, "response_language reset to default %s (saved)\n", config.DefaultResponseLanguage)
	}
	return nil
}
