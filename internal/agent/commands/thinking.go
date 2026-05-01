package commands

import (
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
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

func LegacyTools(d Deps, parts []string) error {
	sess := d.Session()
	if sess == nil {
		return fmt.Errorf("no active session")
	}
	if len(parts) < 2 {
		sess.LegacyTools = !sess.LegacyTools
	} else {
		sw := strings.ToLower(parts[1])
		switch sw {
		case "on", "yes", "true", "1":
			sess.LegacyTools = true
		case "off", "no", "false", "0":
			sess.LegacyTools = false
		default:
			return fmt.Errorf("usage: /legacytools | /legacytools on|off")
		}
	}
	if err := chatstore.WriteSession(d.ProjHex, sess); err != nil {
		return err
	}
	state := "off"
	if sess.LegacyTools {
		state = "on"
	}
	fmt.Fprintf(d.Out, "legacy Tool: line parsing: %s (system prompt includes legacy syntax on next assistant turn)\n", state)
	return nil
}
