package commands

import (
	"fmt"
	"strings"
)

func Name(d Deps, parts []string) error {
	if len(parts) < 2 {
		stored := strings.TrimSpace(d.Cfg.UserName)
		if stored == "" {
			PrintSystem(d.Out, "user_name=(empty)")
		} else {
			PrintSystemf(d.Out, "user_name=%s", stored)
		}
		return nil
	}
	rest := strings.Join(parts[1:], " ")
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return fmt.Errorf("usage: /name | /name <name> | /name clear")
	}
	switch strings.ToLower(rest) {
	case "clear", "default", "reset":
		d.Cfg.UserName = ""
	default:
		d.Cfg.UserName = rest
	}
	if err := d.SaveCfg(); err != nil {
		return err
	}
	if strings.TrimSpace(d.Cfg.UserName) != "" {
		PrintSystemf(d.Out, "user_name=%s (saved; injected into system prompt)", strings.TrimSpace(d.Cfg.UserName))
	} else {
		PrintSystem(d.Out, "user_name cleared (saved)")
	}
	return nil
}
