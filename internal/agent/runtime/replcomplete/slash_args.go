package replcomplete

func slashStaticArgCandidates(cmd string) []string {
	switch cmd {
	case "reasoning":
		return []string{"none", "low", "med", "medium", "high"}
	case "thinking", "terminal", "fast", "cursortools":
		return []string{"on", "off", "yes", "no", "true", "false", "1", "0"}
	case "legacytools", "legacy":
		return []string{"on", "off", "force", "yes", "no", "true", "false", "1", "0"}
	case "log":
		return []string{"error", "warning", "info", "debug", "result"}
	case "add":
		return []string{"rule", "projectrule", "skill"}
	case "remove":
		return []string{"rule", "projectrule", "skill"}
	default:
		return nil
	}
}
