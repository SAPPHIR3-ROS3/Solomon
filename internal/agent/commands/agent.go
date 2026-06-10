package commands

func Agent(d Deps) error {
	d.SetMode("agent")
	PrintSystem(d.Out, "Mode: agent")
	return nil
}

func Chat(d Deps) error {
	d.SetMode("chat")
	PrintSystem(d.Out, "Mode: chat")
	return nil
}
