package commands

import "fmt"

func ExitMessage(d Deps) {
	fmt.Fprintln(d.Out, "Goodbye.")
	s := d.Session()
	if s != nil && s.ID != "" {
		fmt.Fprintf(d.Out, "Resume this chat by id:   /resume %s\n", s.ID)
		if s.Title != "" {
			fmt.Fprintf(d.Out, "Resume this chat by title: /resume %s\n", s.Title)
		}
		return
	}
	if s != nil && len(s.Messages) > 0 {
		fmt.Fprintln(d.Out, "This conversation was not saved (disk write runs after the first completed reply and title).")
		return
	}
	fmt.Fprintln(d.Out, "This chat has no id yet (send a message first). To resume a saved chat:")
	fmt.Fprintln(d.Out, "  /resume              — list recent chats")
	fmt.Fprintln(d.Out, "  /resume last         — open the chat where you sent the last message")
	fmt.Fprintln(d.Out, "  /resume <id>         — open by session id")
	fmt.Fprintln(d.Out, "  /resume <title>      — open by exact title")
}
