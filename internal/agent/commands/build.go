package commands

func Build(d Deps) error {
	PrintSystem(d.Out, modeMigrationMsg)
	d.SetMode("agent")
	PrintSystem(d.Out, "Mode: agent")
	return nil
}
