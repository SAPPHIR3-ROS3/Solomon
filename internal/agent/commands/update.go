package commands

import (
	"fmt"
)

func Update(d Deps) error {
	if err := Clear(d); err != nil {
		return err
	}
	if d.CheckForUpdate == nil {
		PrintSystem(d.Out, "Update check unavailable")
		if d.PrintWelcomeBanner != nil {
			d.PrintWelcomeBanner()
		}
		return nil
	}
	notice, err := d.CheckForUpdate(true)
	if err != nil {
		PrintSystemErr(d.Out, err)
	}
	if notice == nil {
		if err == nil {
			PrintSystemf(d.Out, "Solomon is up to date (%s)", VersionString())
		}
		if d.PrintWelcomeBanner != nil {
			d.PrintWelcomeBanner()
		}
		return nil
	}
	if d.PrintWelcomeBanner != nil {
		d.PrintWelcomeBanner()
	}
	if d.Cfg != nil && d.Cfg.AutoUpdateEnabled() {
		if d.InstallUpdate == nil {
			return fmt.Errorf("/update install unavailable")
		}
		return d.InstallUpdate(notice.Latest)
	}
	return nil
}
