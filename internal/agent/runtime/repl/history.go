package repl

import (
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/multiline"
)

type historyKind int

const (
	historyKindChat historyKind = iota
	historyKindSlash
	historyKindShell
)

type historyEntry struct {
	raw   string
	kind  historyKind
	shell string
}

type inputHistory struct {
	items      []historyEntry
	shellLines []string
	slashLines []string
	idx        int
	draft      string
}

func newInputHistory() *inputHistory {
	return &inputHistory{idx: -1}
}

func classifyReplLine(line string, shellFirst bool) historyEntry {
	e := historyEntry{raw: line, kind: historyKindChat}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return e
	}
	if strings.HasPrefix(trimmed, "/") {
		e.kind = historyKindSlash
		return e
	}
	if shellFirst {
		if strings.HasPrefix(trimmed, "!") {
			return e
		}
		e.kind = historyKindShell
		e.shell = multiline.TrimMessageEdges(trimmed)
		return e
	}
	if strings.HasPrefix(trimmed, "!") {
		cmd := multiline.TrimMessageEdges(strings.TrimPrefix(trimmed, "!"))
		if cmd == "" {
			return e
		}
		e.kind = historyKindShell
		e.shell = cmd
	}
	return e
}

func (h *inputHistory) add(line string, shellFirst bool) {
	if strings.TrimSpace(line) == "" {
		return
	}
	ent := classifyReplLine(line, shellFirst)
	if len(h.items) > 0 && h.items[len(h.items)-1].raw == ent.raw {
		return
	}
	h.items = append(h.items, ent)
	switch ent.kind {
	case historyKindShell:
		if ent.shell != "" {
			h.shellLines = append(h.shellLines, ent.shell)
		}
	case historyKindSlash:
		h.slashLines = append(h.slashLines, ent.raw)
	}
	h.idx = len(h.items)
	h.draft = ""
}

func (h *inputHistory) prev(draft string) (string, bool) {
	if len(h.items) == 0 {
		return "", false
	}
	if h.idx < 0 || h.idx > len(h.items) {
		h.idx = len(h.items)
	}
	if h.idx == len(h.items) {
		h.draft = draft
	}
	if h.idx == 0 {
		return h.items[0].raw, true
	}
	h.idx--
	return h.items[h.idx].raw, true
}

func (h *inputHistory) next() (string, bool) {
	if len(h.items) == 0 || h.idx < 0 || h.idx >= len(h.items) {
		return "", false
	}
	h.idx++
	if h.idx == len(h.items) {
		return h.draft, true
	}
	return h.items[h.idx].raw, true
}

func (h *inputHistory) resetNav() {
	h.idx = len(h.items)
	h.draft = ""
}

func (h *inputHistory) shellMatch(prefix string) string {
	if prefix == "" {
		return ""
	}
	for i := len(h.shellLines) - 1; i >= 0; i-- {
		entry := h.shellLines[i]
		if entry == prefix {
			continue
		}
		if strings.HasPrefix(entry, prefix) {
			return entry
		}
	}
	return ""
}

func (h *inputHistory) slashLinesCopy() []string {
	if len(h.slashLines) == 0 {
		return nil
	}
	out := make([]string, len(h.slashLines))
	copy(out, h.slashLines)
	return out
}
