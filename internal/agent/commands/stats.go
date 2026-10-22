package commands

func Stats(d Deps) error {
	next := !d.Cfg.UsageStatsEnabled()
	d.Cfg.ShowUsageStats = &next
	if err := d.SaveCfg(); err != nil {
		return err
	}
	onOff := "off"
	if next {
		onOff = "on"
	}
	PrintSystemf(d.Out, "token stats: %s", onOff)
	return nil
}

func Changes(d Deps) error {
	next := !d.Cfg.FileChangesEnabled()
	d.Cfg.ShowFileChanges = &next
	if err := d.SaveCfg(); err != nil {
		return err
	}
	PrintSystemf(d.Out, "file change summary: %t", next)
	return nil
}
