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
	if d.MutateSession == nil {
		return fmt.Errorf("no active session")
	}
	var legacy bool
	var usageErr error
	d.MutateSession(func(sess *chatstore.Session) {
		if sess == nil {
			return
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
				usageErr = fmt.Errorf("usage: /legacytools | /legacytools on|off")
				return
			}
		}
		legacy = sess.LegacyTools
	})
	if usageErr != nil {
		return usageErr
	}
	if d.PersistSession != nil {
		if err := d.PersistSession(); err != nil {
			return err
		}
	}
	state := "off"
	if legacy {
		state = "on"
	}
	PrintSystemf(d.Out, "legacy Tool: line parsing: %s (system prompt includes legacy syntax on next assistant turn)", state)
	return nil
}
