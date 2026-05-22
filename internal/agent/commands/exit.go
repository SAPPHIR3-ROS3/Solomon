package commands

import "strings"

func ExitMessage(d Deps) {
	var lines []string
	lines = append(lines, "Goodbye.")
	s := d.Session()
	ephemeral := d.GetEphemeralSession != nil && d.GetEphemeralSession()
	if ephemeral {
		if s != nil && len(s.Messages) > 0 {
			lines = append(lines, "This was a temporary chat (not saved to disk).")
		}
		PrintSystem(d.Out, strings.Join(lines, "\n"))
		return
	}
	if s != nil && s.ID != "" {
		lines = append(lines, "Resume this chat by id:   /resume "+s.ID)
		if s.Title != "" {
			lines = append(lines, "Resume this chat by title: /resume "+s.Title)
		}
		PrintSystem(d.Out, strings.Join(lines, "\n"))
		return
	}
	if s != nil && len(s.Messages) > 0 {
		lines = append(lines, "This conversation was not saved (disk write runs after the first completed reply and title).")
		PrintSystem(d.Out, strings.Join(lines, "\n"))
		return
	}
	lines = append(lines,
		"This chat has no id yet (send a message first). To resume a saved chat:",
		"  /resume              — list recent chats",
		"  /resume last         — open the chat where you sent the last message",
		"  /resume <id>         — open by session id",
		"  /resume <title>      — open by exact title",
	)
	PrintSystem(d.Out, strings.Join(lines, "\n"))
}
