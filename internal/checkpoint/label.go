package checkpoint

import (
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
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

// FormatCheckpointPrefix restituisce il checkpoint con due punti finali (es. "[#001]: ").
// Usato per la prima riga di una tool call (l'intent).
func FormatCheckpointPrefix(cpSeq int, branch string) string {
	if cpSeq < 0 {
		return ""
	}
	t := FormatCheckpointTag(cpSeq, branch)
	if t == "" {
		return ""
	}
	return t + ": "
}

// FormatCheckpointContinuationPlain returns continuation dots whose count matches
// FormatCheckpointTag width (including brackets), plus a trailing space.
func FormatCheckpointContinuationPlain(cpSeq int, branch string) string {
	tag := FormatCheckpointTag(cpSeq, branch)
	if tag == "" {
		return "..... "
	}
	return strings.Repeat(".", len(tag)) + " "
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

	// tutti i messaggi (main + orphans).
	maxIdx := -1
	for _, m := range s.Messages {
		if m.CheckpointSeq == forkAtDisplay && m.CheckpointBranchKey != "" {
			idx := suffixToIndex(m.CheckpointBranchKey)
			if idx > maxIdx {
				maxIdx = idx
			}
		}
	}
	for _, seg := range s.MainOrphans {
		for _, m := range seg.Messages {
			if m.CheckpointSeq == forkAtDisplay && m.CheckpointBranchKey != "" {
				idx := suffixToIndex(m.CheckpointBranchKey)
				if idx > maxIdx {
					maxIdx = idx
				}
			}
		}
	}
	return forkLetterIndex(maxIdx + 1)
}

// suffixToIndex converte un suffisso branch ("a"→0, "z"→25, "aa"→26, ...) in indice.
// Usa lo stesso sistema base-26 senza zero di forkLetterIndex (come le colonne Excel).
func suffixToIndex(s string) int {
	idx := 0
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			idx = idx*26 + int(r-'a'+1)
		}
	}
	return idx - 1
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
