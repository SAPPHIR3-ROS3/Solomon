package commands

func Build(d Deps) error {
	d.SetMode("build")
	PrintSystem(d.Out, "Mode: build")
	return nil
}
