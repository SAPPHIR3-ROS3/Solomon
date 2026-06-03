package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

var ErrRestartSolomon = errors.New("restart solomon")

func Update(d Deps) error {
	if d.CheckForUpdate == nil {
		PrintSystem(d.Out, "Update check unavailable")
		return nil
	}
	notice, err := d.CheckForUpdate(true)
	if printUpToDateIfCurrent(d, notice, err) {
		return nil
	}
	if err := clearTerminal(d); err != nil {
		return err
	}
	if d.PrintWelcomeBanner != nil {
		d.PrintWelcomeBanner()
	}
	printUpdateHints(d, notice)
	return nil
}

func AutoUpdate(d Deps, parts []string) error {
	if d.Cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	next := !d.Cfg.AutoUpdateEnabled()
	if len(parts) >= 2 {
		switch strings.ToLower(parts[1]) {
		case "on", "yes", "true", "1":
			next = true
		case "off", "no", "false", "0":
			next = false
		default:
			return fmt.Errorf("usage: /autoupdate | /autoupdate on|off")
		}
	}
	d.Cfg.AutoUpdate = &next
	if err := d.SaveCfg(); err != nil {
		return err
	}
	onOff := "off"
	if next {
		onOff = "on"
	}
	PrintSystemf(d.Out, "autoupdate: %s (saved to config.toml)", onOff)
	if next {
		PrintSystem(d.Out, "When a newer release is detected, Solomon installs it in the background.")
	} else {
		PrintSystem(d.Out, "Use /upgrade to install when an update is available.")
	}
	return nil
}

func Upgrade(d Deps) error {
	if d.CheckForUpdate == nil {
		return fmt.Errorf("/upgrade unavailable")
	}
	notice, err := d.CheckForUpdate(true)
	if printUpToDateIfCurrent(d, notice, err) {
		return nil
	}
	if d.InstallUpdate == nil {
		return fmt.Errorf("/upgrade install unavailable")
	}
	if cmd, err := updater.InstallCommand(notice.Latest); err == nil {
		PrintSystemf(d.Out, "Installing %s: %s", notice.Latest, cmd)
	} else {
		PrintSystemf(d.Out, "Installing %s...", notice.Latest)
	}
	err = d.InstallUpdate(notice.Latest)
	if errors.Is(err, updater.ErrRestartScheduled) {
		return ErrRestartSolomon
	}
	return err
}

func printUpToDateIfCurrent(d Deps, notice *updater.Notice, err error) bool {
	if notice != nil {
		return false
	}
	if err != nil {
		PrintSystemErr(d.Out, err)
		return true
	}
	PrintSystemf(d.Out, "Solomon is up to date (%s)", VersionString())
	return true
}

func printUpdateHints(d Deps, notice *updater.Notice) {
	PrintSystem(d.Out, "Use /autoupdate on|off to enable or disable automatic installs (config.toml).")
	if d.Cfg != nil && d.Cfg.AutoUpdateEnabled() {
		PrintSystem(d.Out, "autoupdate is on — install runs in the background when a release is newer.")
		return
	}
	PrintSystem(d.Out, "Run /upgrade to install the available release.")
	if notice == nil {
		return
	}
	if cmd, err := updater.InstallCommand(notice.Latest); err == nil {
		PrintSystemf(d.Out, "Manual install: %s", cmd)
	}
}
