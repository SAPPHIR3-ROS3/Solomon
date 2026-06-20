package commands

import (
	"strings"
)

func Btw(d Deps, parts []string) error {
	if len(parts) > 1 && strings.TrimSpace(strings.Join(parts[1:], " ")) != "" {
		PrintSystem(d.Out, "/btw runs during agent generation — type /btw <question> while the model is streaming or tools are running.")
		return nil
	}
	PrintSystem(d.Out, "/btw <question> — side question during generation (type while the agent is running; not available at idle prompt).")
	return nil
}
