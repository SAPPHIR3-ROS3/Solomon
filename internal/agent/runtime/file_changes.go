package agentruntime

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/filechanges"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func (r *Runtime) renderFileChanges(changes []filechanges.Change) {
	added, removed := 0, 0
	for _, c := range changes {
		added += c.Added
		removed += c.Removed
	}
	fmt.Fprintf(r.Out, "\n%s  %s %s\n", termcolor.WrapSystem(fmt.Sprintf("╭─ %d changed files", len(changes))), termcolor.WrapAssistant(fmt.Sprintf("+%d", added)), termcolor.WrapRed(fmt.Sprintf("-%d", removed)))
	folders := map[string]bool{}
	for _, c := range changes {
		parts := strings.Split(c.Name, "/")
		for i := 1; i < len(parts); i++ {
			folder := strings.Join(parts[:i], "/")
			if !folders[folder] {
				folders[folder] = true
				fmt.Fprintf(r.Out, "%s%s\n", termcolor.WrapThinking("│ "+strings.Repeat("  ", i-1)), termcolor.WrapTool(safeChangeLabel(parts[i-1])+"/"))
			}
		}
		label := parts[len(parts)-1]
		if strings.ContainsAny(label, "\x1b\r\n\t") {
			label = strconv.Quote(label)
		}
		counts := termcolor.WrapAssistant(fmt.Sprintf("+%d", c.Added)) + " " + termcolor.WrapRed(fmt.Sprintf("-%d", c.Removed))
		if c.Binary {
			counts = termcolor.WrapThinking("binary")
		}
		fmt.Fprintf(r.Out, "%s%s  %s\n", termcolor.WrapThinking("│ "+strings.Repeat("  ", len(parts)-1)), termcolor.WrapWhite(label), counts)
	}
	fmt.Fprintln(r.Out, termcolor.WrapSystem("╰─"))
	flushWriter(r.Out)
}

func (r *Runtime) recordChatFileChanges(seq int, changes []filechanges.Change, committed []filechanges.Commit) {
	r.mutateSession(func(s *chatstore.Session) {
		turn := seq
		for i := len(s.Messages) - 1; i >= 0; i-- {
			if s.Messages[i].Role == "user" {
				turn = s.Messages[i].CheckpointSeq
				break
			}
		}
		// Project edits accumulate until committed; external edits belong only
		// to the turn that made them. Retain accumulation within the same turn.
		if s.FileChangesTurn != turn {
			for name := range s.FileChanges {
				if filepath.IsAbs(name) {
					delete(s.FileChanges, name)
				}
			}
		}
		if s.FileChanges == nil {
			s.FileChanges = map[string]chatstore.FileChangeLedger{}
		}
		for _, c := range changes {
			entry, exists := s.FileChanges[c.Name]
			if !exists {
				entry = chatstore.FileChangeLedger{BeforeExists: c.BeforeExists}
			}
			if entry.AddedLines == nil {
				entry.AddedLines = map[string]int{}
			}
			if entry.RemovedLines == nil {
				entry.RemovedLines = map[string]int{}
			}
			adds, removes := c.AddedLines, c.RemovedLines
			if adds == nil {
				adds = map[string]int{}
				removes = map[string]int{}
				for line, delta := range c.Lines {
					if delta > 0 {
						adds[line] = delta
					} else {
						removes[line] = -delta
					}
				}
			}
			// Cancel against contributions from earlier operations only. A move
			// within this operation must retain both its addition and deletion.
			nextAdds, nextRemoves := map[string]int{}, map[string]int{}
			for line, count := range adds {
				cancel := min(count, entry.RemovedLines[line])
				entry.RemovedLines[line] -= cancel
				nextAdds[line] = count - cancel
			}
			for line, count := range removes {
				cancel := min(count, entry.AddedLines[line])
				entry.AddedLines[line] -= cancel
				nextRemoves[line] = count - cancel
			}
			for line, count := range nextAdds {
				entry.AddedLines[line] += count
			}
			for line, count := range nextRemoves {
				entry.RemovedLines[line] += count
			}
			cleanLineCounts(&entry)
			entry.AfterExists = c.AfterExists
			entry.Binary = entry.Binary || c.Binary
			if !entry.Binary && len(entry.AddedLines) == 0 && len(entry.RemovedLines) == 0 && entry.BeforeExists == entry.AfterExists {
				delete(s.FileChanges, c.Name)
			} else {
				s.FileChanges[c.Name] = entry
			}
		}
		for _, commit := range committed {
			if commit.Clean {
				delete(s.FileChanges, commit.Name)
				continue
			}
			entry, ok := s.FileChanges[commit.Name]
			if !ok {
				continue
			}
			for line, delta := range commit.Lines {
				if delta > 0 {
					if count := entry.AddedLines[line]; count > 0 {
						entry.AddedLines[line] -= min(delta, count)
					}
				}
				if delta < 0 {
					if count := entry.RemovedLines[line]; count > 0 {
						entry.RemovedLines[line] -= min(-delta, count)
					}
				}
			}
			cleanLineCounts(&entry)
			if !entry.Binary && len(entry.AddedLines) == 0 && len(entry.RemovedLines) == 0 {
				delete(s.FileChanges, commit.Name)
			} else {
				s.FileChanges[commit.Name] = entry
			}
		}
		if len(changes) > 0 || len(committed) > 0 {
			s.FileChangesTurn = turn
		}
	})
}
func (r *Runtime) printChatFileChanges(seq int) {
	var changes []filechanges.Change
	r.mutateSession(func(s *chatstore.Session) {
		if s.FileChangesTurn != seq {
			return
		}
		for name, entry := range s.FileChanges {
			c := filechanges.Change{Name: name, Binary: entry.Binary}
			for _, count := range entry.AddedLines {
				c.Added += count
			}
			for _, count := range entry.RemovedLines {
				c.Removed += count
			}
			changes = append(changes, c)
		}
	})
	if len(changes) == 0 {
		return
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Name < changes[j].Name })
	r.renderFileChanges(changes)
}

func safeChangeLabel(label string) string {
	if strings.ContainsAny(label, "\x1b\r\n\t") {
		return strconv.Quote(label)
	}
	return label
}

func cleanLineCounts(entry *chatstore.FileChangeLedger) {
	for line, count := range entry.AddedLines {
		if count == 0 {
			delete(entry.AddedLines, line)
		}
	}
	for line, count := range entry.RemovedLines {
		if count == 0 {
			delete(entry.RemovedLines, line)
		}
	}
}

func (r *Runtime) saveTurnFileChanges(seq int) error {
	changed := false
	r.mutateSession(func(s *chatstore.Session) {
		if s == nil || s.FileChangesTurn != seq {
			return
		}
		var summary []chatstore.FileChangeSummary
		for name, entry := range s.FileChanges {
			c := chatstore.FileChangeSummary{Path: name, Binary: entry.Binary}
			for line, count := range entry.AddedLines {
				c.Added += count
				for i := 0; i < count; i++ {
					c.AddedLines = append(c.AddedLines, line)
				}
			}
			for line, count := range entry.RemovedLines {
				c.Removed += count
				for i := 0; i < count; i++ {
					c.RemovedLines = append(c.RemovedLines, line)
				}
			}
			sort.Strings(c.AddedLines)
			sort.Strings(c.RemovedLines)
			summary = append(summary, c)
		}
		sort.Slice(summary, func(i, j int) bool { return summary[i].Path < summary[j].Path })
		for i := len(s.Messages) - 1; i >= 0; i-- {
			if s.Messages[i].Role == "user" {
				break
			}
			if s.Messages[i].Role == "assistant" {
				s.Messages[i].FileChanges = summary
				s.Messages[i].FileChangesRecorded = true
				changed = true
				break
			}
		}
	})
	if changed {
		return r.persistSession()
	}
	return nil
}
