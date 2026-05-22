package commands

func Plan(d Deps) error {
	d.SetMode("plan")
	PrintSystem(d.Out, "Mode: plan")
	return nil
}
