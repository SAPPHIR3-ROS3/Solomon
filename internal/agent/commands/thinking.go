package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	cursorint "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor"
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

func Fast(d Deps, parts []string) error {
	if d.Cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	next := !d.Cfg.EffectiveFastMode()
	if len(parts) >= 2 {
		switch strings.ToLower(parts[1]) {
		case "on", "yes", "true", "1":
			next = true
		case "off", "no", "false", "0":
			next = false
		default:
			return fmt.Errorf("usage: /fast | /fast on|off")
		}
	}
	d.Cfg.FastMode = &next
	if err := d.SaveCfg(); err != nil {
		return err
	}
	onOff := "off"
	if next {
		onOff = "on"
	}
	PrintSystemf(d.Out, "fast mode: %s", onOff)
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

func CursorTools(d Deps, parts []string) error {
	if d.Cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	if !config.CursorAPIConfigured(d.Cfg) {
		return fmt.Errorf("/cursortools unavailable (connect Cursor API first with /connect)")
	}
	next := !d.Cfg.Tools.CursorInternalTools
	if len(parts) >= 2 {
		switch strings.ToLower(parts[1]) {
		case "on", "yes", "true", "1":
			next = true
		case "off", "no", "false", "0":
			next = false
		default:
			return fmt.Errorf("usage: /cursortools | /cursortools on|off")
		}
	}
	d.Cfg.Tools.CursorInternalTools = next
	if err := d.SaveCfg(); err != nil {
		return err
	}
	ctx := d.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	cwd := d.ProjRoot
	if strings.TrimSpace(cwd) == "" {
		cwd, _ = os.Getwd()
	}
	cursorint.KickSidecarIfConfigured(ctx, d.Cfg, cwd, cursorint.DiscardBootstrap{})
	onOff := "off"
	if next {
		onOff = "on"
	}
	PrintSystemf(d.Out, "cursor native tools: %s", onOff)
	return nil
}
