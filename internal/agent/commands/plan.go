package commands

const modeMigrationMsg = "Mode /plan and /build are deprecated; use /agent (implementation) or /chat (web/docs). Switching to /agent."

func Plan(d Deps) error {
	PrintSystem(d.Out, modeMigrationMsg)
	d.SetMode("agent")
	PrintSystem(d.Out, "Mode: agent")
	return nil
}
