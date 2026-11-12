package interaction

import (
	"strings"
	"sync"
	"unicode/utf8"
)

func validID(id string) bool {
	if id == "" {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == ':' || c == '.') {
			return false
		}
	}
	return true
}

func Mark(id, body string) string {
	if !validID(id) {
		return body
	}
	return "\x1b]777;zone;start;" + id + "\x1b\\" + body + "\x1b]777;zone;end;" + id + "\x1b\\"
}

type region struct {
	id               string
	x0, x1, y, order int
}

type Map struct {
	mu      sync.RWMutex
	regions []region
}

type zone struct {
	id       string
	x, order int
}

func (m *Map) Hit(x, y int) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	best, id := -1, ""
	for _, r := range m.regions {
		if r.y == y && x >= r.x0 && x < r.x1 && r.order > best {
			best, id = r.order, r.id
		}
	}
	return id
}

func (m *Map) Capture(view string, measure func(string) int) string {
	if measure == nil {
		measure = utf8.RuneCountInString
	}
	var output, line strings.Builder
	var regions []region
	var active []zone
	y, order := 0, 0
	position := func() int { return measure(line.String()) }
	flush := func(x int) {
		for i := range active {
			z := &active[i]
			if x > z.x {
				regions = append(regions, region{z.id, z.x, x, y, z.order})
			}
			z.x = x
		}
	}
	for i := 0; i < len(view); {
		if view[i] == '\x1b' {
			end, payload, osc := escape(view, i)
			token := view[i:end]
			action, id, ok := marker(payload)
			if osc && ok {
				x := position()
				if action == "start" {
					order++
					active = append(active, zone{id, x, order})
				} else {
					for j := len(active) - 1; j >= 0; j-- {
						if active[j].id == id {
							z := active[j]
							if x > z.x {
								regions = append(regions, region{z.id, z.x, x, y, z.order})
							}
							active = append(active[:j], active[j+1:]...)
							break
						}
					}
				}
			} else {
				output.WriteString(token)
			}
			i = end
			continue
		}
		c := view[i]
		if c == '\n' || c == '\r' {
			if len(active) > 0 {
				flush(position())
			}
			output.WriteByte(c)
			line.Reset()
			if c == '\n' {
				y++
			}
			for j := range active {
				active[j].x = 0
			}
			i++
			continue
		}
		start := i
		for i < len(view) && view[i] != '\x1b' && view[i] != '\n' && view[i] != '\r' {
			i++
		}
		output.WriteString(view[start:i])
		line.WriteString(view[start:i])
	}
	if len(active) > 0 {
		flush(position())
	}
	m.mu.Lock()
	m.regions = regions
	m.mu.Unlock()
	return output.String()
}

func marker(payload string) (string, string, bool) {
	rest, ok := strings.CutPrefix(payload, "777;zone;")
	if !ok {
		return "", "", false
	}
	action, id, ok := strings.Cut(rest, ";")
	return action, id, ok && (action == "start" || action == "end") && validID(id)
}

func escape(s string, start int) (int, string, bool) {
	if start+1 >= len(s) {
		return len(s), "", false
	}
	switch s[start+1] {
	case ']':
		for i := start + 2; i < len(s); i++ {
			if s[i] == '\a' {
				return i + 1, s[start+2 : i], true
			}
			if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2, s[start+2 : i], true
			}
		}
		return len(s), "", false
	case '[':
		for i := start + 2; i < len(s); i++ {
			if s[i] >= 0x40 && s[i] <= 0x7e {
				return i + 1, "", false
			}
		}
		return len(s), "", false
	default:
		return start + 2, "", false
	}
}
