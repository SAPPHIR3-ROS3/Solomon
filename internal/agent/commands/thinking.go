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
		PrintSystemf(d.Out, "streaming reasoning preview: %s", onOff)
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
	PrintSystemf(d.Out, "streaming reasoning preview: %s", onOff)
	return nil
}

func LegacyTools(d Deps, parts []string) error {
	if d.Cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	if len(parts) < 2 {
		d.Cfg.Tools.Legacy = !d.Cfg.Tools.Legacy
		if !d.Cfg.Tools.Legacy {
			d.Cfg.Tools.LegacyForce = false
		}
	} else if strings.EqualFold(parts[1], "force") {
		if len(parts) < 3 {
			d.Cfg.Tools.LegacyForce = !d.Cfg.Tools.LegacyForce
		} else {
			switch strings.ToLower(parts[2]) {
			case "on", "yes", "true", "1":
				d.Cfg.Tools.LegacyForce = true
			case "off", "no", "false", "0":
				d.Cfg.Tools.LegacyForce = false
			default:
				return fmt.Errorf("usage: /legacytools force | /legacytools force on|off")
			}
		}
		if d.Cfg.Tools.LegacyForce {
			d.Cfg.Tools.Legacy = true
		}
	} else {
		switch strings.ToLower(parts[1]) {
		case "on", "yes", "true", "1":
			d.Cfg.Tools.Legacy = true
		case "off", "no", "false", "0":
			d.Cfg.Tools.Legacy = false
			d.Cfg.Tools.LegacyForce = false
		default:
			return fmt.Errorf("usage: /legacytools | /legacytools on|off | /legacytools force | /legacytools force on|off")
		}
	}
	if err := d.SaveCfg(); err != nil {
		return err
	}
	legacyState := "OFF"
	if d.Cfg.Tools.Legacy {
		legacyState = "ON"
	}
	forceState := "OFF"
	if d.Cfg.Tools.LegacyForce {
		forceState = "ON"
	}
	PrintSystemf(d.Out, "legacy tools: %s, force: %s", legacyState, forceState)
	return nil
}
