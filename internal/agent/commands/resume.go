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

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func sessionIsEmpty(sess *chatstore.Session) bool {
	return sess == nil || len(sess.Messages) == 0
}

func Resume(d Deps, args []string) error {
	if d.SetEphemeralSession != nil {
		d.SetEphemeralSession(false)
	}
	if len(args) == 0 {
		list, err := chatstore.ListRecent(d.ProjHex, 10)
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		for i, s := range list {
			fmt.Fprintf(&buf, "%d\t%s\t%s\n", i, s.ID, s.Title)
		}
		buf.WriteString("pick number or /resume <id|title> or /resume last")
		PrintSystem(d.Out, buf.String())
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
		PrintSystemf(d.Out, "loaded chat %s (latest user message)", sess.ID)
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
	PrintSystemf(d.Out, "loaded chat %s", sess.ID)
	afterResumeLoaded(d, sess)
	return nil
}

func afterResumeLoaded(d Deps, sess *chatstore.Session) {
	chatstore.FinishSessionLoad(sess)
	model := d.Model()
	if d.Cfg != nil {
		model = d.Cfg.ModelDisplayName(d.Provider(), model)
	}
	printResumedTranscript(d.Out, sess, model, usageStatsEnabled(d))
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

func printResumedTranscript(out io.Writer, sess *chatstore.Session, model string, showUsage bool) {
	if len(sess.Messages) == 0 {
		return
	}
	fmt.Fprintln(out)
	WriteLabeledTranscript(out, sess.Messages, model, showUsage)
	fmt.Fprintln(out)
}

func usageStatsEnabled(d Deps) bool {
	return d.Cfg != nil && d.Cfg.UsageStatsEnabled()
}

func NewChat(d Deps) error {
	if d.SetEphemeralSession != nil {
		d.SetEphemeralSession(false)
	}
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
		ImageSeq:               0,
		ImageFiles:             nil,
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

func TempChat(d Deps) error {
	if d.SetEphemeralSession == nil {
		return fmt.Errorf("/temp unavailable")
	}
	sess := d.Session()
	if !sessionIsEmpty(sess) {
		return fmt.Errorf("cannot create a temporary chat: the current chat already has messages")
	}
	d.SetEphemeralSession(true)
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
		ImageSeq:               0,
		ImageFiles:             nil,
	})
	if d.ResetReadlineHistory != nil {
		d.ResetReadlineHistory()
	}
	fmt.Fprint(d.Out, "\033[2J\033[H")
	if d.PrintWelcomeBanner != nil {
		d.PrintWelcomeBanner()
	}
	PrintSystem(d.Out, "temp session (in memory only; not saved to disk)")
	return nil
}
