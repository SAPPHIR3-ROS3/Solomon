package tooling

import (
	"encoding/json"
	"strings"
)

type Invocation struct {
	Name string
	Args json.RawMessage
}

func ExtractToolInvocations(text string) []Invocation {
	var invs []Invocation
	lines := strings.Split(text, "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || !strings.HasPrefix(line, "Tool:") {
			continue
		}
		rest := strings.TrimSpace(line[len("Tool:"):])
		open := strings.Index(rest, "(")
		closeIdx := strings.LastIndex(rest, ")")
		if open < 0 || closeIdx <= open {
			continue
		}
		name := strings.TrimSpace(rest[:open])
		jsonStr := strings.TrimSpace(rest[open+1 : closeIdx])
		if name == "" {
			continue
		}
		if !json.Valid([]byte(jsonStr)) {
			continue
		}
		invs = append(invs, Invocation{Name: name, Args: json.RawMessage(jsonStr)})
	}
	return invs
}
