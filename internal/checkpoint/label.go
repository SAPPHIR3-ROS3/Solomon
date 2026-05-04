package checkpoint

import (
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
)

func FormatCheckpointTag(cpSeq int, branch string) string {
	if cpSeq < 0 {
		return ""
	}
	if branch != "" {
		return fmt.Sprintf("[#%03d%s]", cpSeq, branch)
	}
	return fmt.Sprintf("[#%03d]", cpSeq)
}

func FormatLinePrefix(cpSeq int, branch string) string {
	if cpSeq < 0 {
		return ""
	}
	t := FormatCheckpointTag(cpSeq, branch)
	if t == "" {
		return ""
	}
	return t + " "
}

func FormatReplPromptPrefix(s *chatstore.Session) string {
	if s == nil {
		return FormatLinePrefix(0, "")
	}
	next := s.CheckpointLast + 1
	if next < 0 {
		next = 0
	}
	return FormatLinePrefix(next, s.CheckpointBranchSuffix)
}

func NextForkSuffix(s *chatstore.Session, forkAtDisplay int) string {
	if forkAtDisplay < 0 {
		return ""
	}
	if s.ForkChildCount == nil {
		s.ForkChildCount = map[int]int{}
	}
	s.ForkChildCount[forkAtDisplay]++
	idx := s.ForkChildCount[forkAtDisplay] - 1
	return forkLetterIndex(idx)
}

func forkLetterIndex(i int) string {
	if i < 26 {
		return string(rune('a' + i))
	}
	var out string
	n := i
	for n >= 0 {
		out = string(rune('a'+(n%26))) + out
		n = n/26 - 1
	}
	return out
}

func Bump(s *chatstore.Session) int {
	s.CheckpointLast++
	return s.CheckpointLast
}

func StampMsg(m *chatstore.Message, s *chatstore.Session, seq int) {
	if m == nil {
		return
	}
	m.CheckpointSeq = seq
	m.CpSeqSet = true
	m.CheckpointBranchKey = s.CheckpointBranchSuffix
}
