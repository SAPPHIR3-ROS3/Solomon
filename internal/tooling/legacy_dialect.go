package tooling

import (
	"encoding/json"
	"regexp"
	"strings"
)

var reFunctionCallJSON = regexp.MustCompile(`(?is)<functioncall>\s*(\{.*?\})\s*(?:</functioncall>|$)`)

func normalizeLegacyToolBlock(block string) string {
	block = strings.TrimSpace(block)
	if block == "" {
		return block
	}
	inner := block
	if strings.HasPrefix(strings.ToLower(block), strings.ToLower(tagToolCallsOpen)) {
		if !strings.HasSuffix(strings.ToLower(block), strings.ToLower(tagToolCallsClose)) {
			inner = strings.TrimSpace(block[len(tagToolCallsOpen):])
		} else {
			inner = strings.TrimSpace(block[len(tagToolCallsOpen) : len(block)-len(tagToolCallsClose)])
		}
		inner = normalizeLegacyToolInner(inner)
		return tagToolCallsOpen + inner + tagToolCallsClose
	}
	inner = normalizeLegacyToolInner(block)
	return tagToolCallsOpen + inner + tagToolCallsClose
}

func normalizeLegacyToolInner(inner string) string {
	s := strings.TrimSpace(inner)
	s = convertJSONToolCallTags(s)
	s = reFunctionCallJSON.ReplaceAllStringFunc(s, func(m string) string {
		sub := reFunctionCallJSON.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		if conv := solomonToolFromJSONObject(sub[1]); conv != "" {
			return conv
		}
		return m
	})
	s = strings.ReplaceAll(s, "</tool_call>", "</tool>")
	s = stripStrayToolCallBeforeNamedTool(s)
	s = strings.ReplaceAll(s, "<tool_call>", "<tool>")
	open := strings.Count(strings.ToLower(s), "<tool>")
	close := strings.Count(s, "</tool>")
	for close < open {
		s += "</tool>"
		close++
	}
	return s
}

func convertJSONToolCallTags(s string) string {
	lower := strings.ToLower(s)
	var b strings.Builder
	for {
		idx := strings.Index(lower, "<tool_call>")
		if idx < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:idx])
		s = s[idx:]
		lower = lower[idx:]
		openEnd := strings.Index(lower, ">")
		if openEnd < 0 {
			b.WriteString(s)
			break
		}
		innerStart := openEnd + 1
		closeRel := strings.Index(lower[innerStart:], "</tool_call>")
		if closeRel < 0 {
			b.WriteString(s)
			break
		}
		inner := strings.TrimSpace(s[innerStart : innerStart+closeRel])
		rest := s[innerStart+closeRel+len("</tool_call>"):]
		if conv := solomonToolFromJSONObject(inner); conv != "" {
			b.WriteString(conv)
		} else {
			b.WriteString(normalizeLegacyToolInner(inner))
		}
		s = rest
		lower = strings.ToLower(s)
	}
	return b.String()
}

type legacyToolCallJSON struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	Args      json.RawMessage `json:"args"`
}

func solomonToolFromJSONObject(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw[0] != '{' {
		return ""
	}
	var tc legacyToolCallJSON
	if err := json.Unmarshal([]byte(raw), &tc); err != nil {
		return ""
	}
	name := strings.TrimSpace(tc.Name)
	if name == "" {
		return ""
	}
	args := tc.Arguments
	if len(args) == 0 {
		args = tc.Args
	}
	args = normalizeLegacyToolArgsJSON(args)
	if len(args) == 0 || !json.Valid(args) {
		return ""
	}
	return `<tool name="` + escapeToolXMLAttr(name) + `"><args>` + string(args) + `</args></tool>`
}

func normalizeLegacyToolArgsJSON(args json.RawMessage) json.RawMessage {
	args = json.RawMessage(strings.TrimSpace(string(args)))
	if len(args) == 0 {
		return json.RawMessage(`{}`)
	}
	if args[0] == '{' {
		return args
	}
	if args[0] == '"' {
		var s string
		if json.Unmarshal(args, &s) == nil {
			inner := strings.TrimSpace(s)
			if inner != "" && inner[0] == '{' && json.Valid([]byte(inner)) {
				return json.RawMessage(inner)
			}
		}
	}
	return nil
}

func stripStrayToolCallBeforeNamedTool(s string) string {
	lower := strings.ToLower(s)
	for {
		idx := strings.Index(lower, "<tool_call>")
		if idx < 0 {
			return s
		}
		after := strings.TrimLeft(s[idx+len("<tool_call>"):], " \t\r\n")
		if len(after) >= len("<tool ") && strings.EqualFold(after[:len("<tool ")], "<tool ") {
			s = s[:idx] + s[idx+len("<tool_call>"):]
			lower = strings.ToLower(s)
			continue
		}
		return s
	}
}

func escapeToolXMLAttr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	return s
}

func extractAlternateToolBlock(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	lower := strings.ToLower(text)
	candidates := []struct {
		open  string
		close string
	}{
		{"<tool_calls>", "</tool_calls>"},
		{"<tool_call>", "</tool_call>"},
		{"<functioncall>", "</functioncall>"},
	}
	bestStart := -1
	bestEnd := -1
	for _, c := range candidates {
		start := strings.Index(lower, c.open)
		if start < 0 {
			continue
		}
		end := strings.LastIndex(lower, c.close)
		if end < start {
			if c.open == "<tool_calls>" || c.open == "<tool_call>" {
				if name := strings.Index(lower[start:], `<tool name=`); name >= 0 {
					end = findLegacyToolsRegionEnd(text, start)
				}
			}
			if end < start {
				continue
			}
		} else {
			end += len(c.close)
		}
		if bestStart < 0 || start < bestStart {
			bestStart = start
			bestEnd = end
		}
	}
	if bestStart < 0 {
		if idx := strings.Index(lower, `<tool name=`); idx >= 0 {
			bestStart = idx
			bestEnd = findLegacyToolsRegionEnd(text, idx)
		}
	}
	if bestStart < 0 || bestEnd <= bestStart {
		return "", false
	}
	fragment := strings.TrimSpace(text[bestStart:bestEnd])
	return normalizeLegacyToolBlock(fragment), true
}

func findLegacyToolsRegionEnd(text string, from int) int {
	lower := strings.ToLower(text[from:])
	lastClose := strings.LastIndex(lower, "</tool>")
	if lastClose >= 0 {
		return from + lastClose + len("</tool>")
	}
	return len(text)
}

func stripLegacyToolRegions(text string) string {
	for {
		changed := false
		for _, pair := range []struct{ open, close string }{
			{tagToolCallsOpen, tagToolCallsClose},
			{"<tool_call>", "</tool_call>"},
			{"<functioncall>", "</functioncall>"},
		} {
			open := strings.Index(strings.ToLower(text), strings.ToLower(pair.open))
			if open < 0 {
				continue
			}
			closeRel := strings.Index(strings.ToLower(text[open:]), strings.ToLower(pair.close))
			if closeRel < 0 {
				if pair.open != tagToolCallsOpen {
					continue
				}
				text = strings.TrimSpace(text[:open])
				changed = true
				break
			}
			close := open + closeRel + len(pair.close)
			before := strings.TrimSpace(text[:open])
			after := strings.TrimSpace(text[close:])
			switch {
			case before != "" && after != "":
				text = before + "\n" + after
			case before != "":
				text = before
			default:
				text = after
			}
			changed = true
			break
		}
		if !changed {
			break
		}
	}
	return strings.TrimSpace(text)
}
