package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"solomon/internal/chatstore"
)

func Resume(d Deps, args []string) error {
	if len(args) == 0 {
		list, err := chatstore.ListRecent(d.ProjHex, 10)
		if err != nil {
			return err
		}
		for i, s := range list {
			fmt.Fprintf(d.Out, "%d\t%s\t%s\n", i, s.ID, s.Title)
		}
		fmt.Fprint(d.Out, "pick number or /resume <id|title> or /resume last\n")
		return nil
	}
	arg := strings.TrimSpace(args[0])
	if strings.EqualFold(arg, "last") {
		sess, err := chatstore.SessionWithLatestUserMessage(d.ProjHex)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("no saved chats yet")
			}
			return err
		}
		d.SetSession(sess)
		fmt.Fprintf(d.Out, "loaded chat %s (latest user message)\n", sess.ID)
		return nil
	}
	sess, err := chatstore.ReadSession(d.ProjHex, arg)
	if err != nil {
		sess, err = chatstore.FindByTitle(d.ProjHex, arg)
	}
	if err != nil {
		return err
	}
	d.SetSession(sess)
	fmt.Fprintf(d.Out, "loaded chat %s\n", sess.ID)
	return nil
}
