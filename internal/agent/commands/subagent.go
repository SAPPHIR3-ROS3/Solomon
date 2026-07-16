package commands

import (
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func Subagent(d Deps, parts []string) error {
	if len(parts) < 2 {
		return subagentList(d)
	}
	sub := strings.ToLower(parts[1])
	switch sub {
	case "resume", "stop", "cancel":
		if len(parts) < 3 {
			return fmt.Errorf("usage: /subagent %s <id|title>", sub)
		}
		return subagentCommand(d, sub, strings.Join(parts[2:], " "))
	default:
		return subagentCommand(d, sub, strings.Join(parts[2:], " "))
	}
}

func subagentList(d Deps) error {
	sessions, err := chatstore.ListSubSessions(d.ProjHex)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		PrintSystem(d.Out, "no subagents")
		return nil
	}
	type row struct {
		n      int
		title  string
		status string
		id     string
	}
	rows := make([]row, 0, len(sessions))
	maxTitle := 0
	maxID := 0
	for i, s := range sessions {
		title := s.Title
		if title == "" {
			title = s.ID
		}
		if len(title) > maxTitle {
			maxTitle = len(title)
		}
		if len(s.ID) > maxID {
			maxID = len(s.ID)
		}
		rows = append(rows, row{n: i + 1, title: title, status: s.Status, id: s.ID})
	}
	col := maxTitle
	if maxID > col {
		col = maxID
	}
	for _, r := range rows {
		line1 := fmt.Sprintf("%d. %s %s", r.n, padRight(r.title, col), r.status)
		fmt.Fprintln(d.Out, termcolor.WrapSystem(line1))
		fmt.Fprintln(d.Out, termcolor.WrapSystem("   "+r.id))
	}
	return nil
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

func subagentCommand(d Deps, cmd, target string) error {
	sessions, err := chatstore.ListSubSessions(d.ProjHex)
	if err != nil {
		return err
	}
	var sess *chatstore.SubSession
	target = strings.TrimSpace(target)
	for _, s := range sessions {
		if s.ID == target || strings.EqualFold(s.Title, target) {
			sess = s
			break
		}
	}
	if sess == nil {
		return fmt.Errorf("subagent not found: %s", target)
	}
	switch cmd {
	case "stop":
		if d.ControlSubagent != nil {
			return d.ControlSubagent(sess.ID, cmd)
		}
		sess.Status = chatstore.SubStatusPaused
		return chatstore.WriteSubSession(sess.ProjectHex, sess)
	case "cancel":
		if d.ControlSubagent != nil {
			return d.ControlSubagent(sess.ID, cmd)
		}
		sess.Status = chatstore.SubStatusCancelled
		return chatstore.WriteSubSession(sess.ProjectHex, sess)
	case "resume":
		if d.ControlSubagent != nil {
			return d.ControlSubagent(sess.ID, cmd)
		}
		if chatstore.SubSessionRunning(sess.Status) {
			return fmt.Errorf("subagent %s is running; stop first", sess.ID)
		}
		sess.Status = chatstore.SubStatusRunning
		if err := chatstore.WriteSubSession(sess.ProjectHex, sess); err != nil {
			return err
		}
		PrintSystemf(d.Out, "subagent %s marked running; use tool resume=%s to continue", sess.Title, sess.ID)
		return nil
	default:
		return fmt.Errorf("unknown /subagent command: %s", cmd)
	}
}
