package replcomplete

import "strings"

func reasoningCompletionArgs(line []rune, start, end int) (int, []string) {
	args := string(line[start:end])
	if i := strings.IndexAny(args, " \t"); i >= 0 {
		if strings.EqualFold(args[:i], "subagent") || strings.EqualFold(args[:i], "sub") {
			start += i
			for start < end && (line[start] == ' ' || line[start] == '\t') {
				start++
			}
			return start, append(reasoningEffortCandidates(), "reset")
		}
	}
	return start, append(reasoningEffortCandidates(), "subagent")
}

func reasoningEffortCandidates() []string {
	return []string{"none", "low", "med", "medium", "high", "xhigh", "extra-high", "max"}
}

func slashStaticArgCandidates(cmd string) []string {
	switch cmd {
	case "thinking", "terminal", "fast", "anonymizeprompt", "cursortools":
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
