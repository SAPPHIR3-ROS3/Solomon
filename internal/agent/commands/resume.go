package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
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
		afterResumeLoaded(d, sess)
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
	afterResumeLoaded(d, sess)
	return nil
}

func afterResumeLoaded(d Deps, sess *chatstore.Session) {
	chatstore.FinishSessionLoad(sess)
	printResumedTranscript(d.Out, sess, d.Model())
	syncReadlineHistoryFromSession(d, sess)
}

func userPromptLines(msgs []chatstore.Message) []string {
	var out []string
	for _, m := range msgs {
		if m.Role != "user" || strings.TrimSpace(m.Content) == "" {
			continue
		}
		if strings.HasPrefix(m.Content, "tool_result(") {
			continue
		}
		out = append(out, m.Content)
	}
	return out
}

func syncReadlineHistoryFromSession(d Deps, sess *chatstore.Session) {
	if d.ResetReadlineHistory != nil {
		d.ResetReadlineHistory()
	}
	if d.AppendReadlineHistory == nil {
		return
	}
	for _, line := range userPromptLines(sess.Messages) {
		_ = d.AppendReadlineHistory(line)
	}
}

func compactJSONArgs(s string) string {
	if s == "" || !json.Valid([]byte(s)) {
		return s
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(s)); err != nil {
		return s
	}
	return buf.String()
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func printResumedTranscript(out io.Writer, sess *chatstore.Session, model string) {
	if len(sess.Messages) == 0 {
		return
	}
	fmt.Fprintln(out)
	WriteLabeledTranscript(out, sess.Messages, model)
	fmt.Fprintln(out)
}

func NewChat(d Deps) error {
	now := time.Now()
	d.SetSession(&chatstore.Session{
		ID:                     "",
		Title:                  "",
		CreatedAt:              now,
		LastMessageAt:          now,
		Messages:               nil,
		CheckpointLast:         -1,
		CheckpointCP0:          true,
		CheckpointBranchSuffix: "",
		ForkChildCount:         nil,
		MainOrphans:            nil,
		LastCommitOID:          "",
	})
	if d.ResetReadlineHistory != nil {
		d.ResetReadlineHistory()
	}
	fmt.Fprint(d.Out, "\033[2J\033[H")
	if d.PrintWelcomeBanner != nil {
		d.PrintWelcomeBanner()
	}
	return nil
}
