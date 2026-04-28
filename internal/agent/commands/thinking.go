package commands

import (
	"fmt"
	"strings"
)

func Thinking(d Deps, parts []string) error {
	if len(parts) < 2 {
		d.Cfg.ShowThinking = !d.Cfg.ShowThinking
		if err := d.SaveCfg(); err != nil {
			return err
		}
		onOff := "off"
		if d.Cfg.ShowThinking {
			onOff = "on"
		}
		fmt.Fprintf(d.Out, "streaming reasoning preview: %s\n", onOff)
		return nil
	}
	sw := strings.ToLower(parts[1])
	switch sw {
	case "on", "yes", "true", "show", "1":
		d.Cfg.ShowThinking = true
	case "off", "no", "false", "hide", "0":
		d.Cfg.ShowThinking = false
	default:
		return fmt.Errorf("usage: /thinking | /thinking on|off")
	}
	if err := d.SaveCfg(); err != nil {
		return err
	}
	onOff := "off"
	if d.Cfg.ShowThinking {
		onOff = "on"
	}
	fmt.Fprintf(d.Out, "streaming reasoning preview: %s\n", onOff)
	return nil
}
