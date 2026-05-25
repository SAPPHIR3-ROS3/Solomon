package tooling

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	tagToolCallsOpen  = "<tool_calls>"
	tagToolCallsClose = "</tool_calls>"
)

var (
	reToolOpen  = regexp.MustCompile(`<tool\s+name="([^"]*)"\s*>`)
	reIntent    = regexp.MustCompile(`(?s)<intent>(.*?)</intent>`)
	reArgs      = regexp.MustCompile(`(?s)<args>(.*?)</args>`)
	reToolClose = regexp.MustCompile(`</tool>`)
)

func ParseToolCallsBlock(block string) ([]Invocation, error) {
	block = strings.TrimSpace(block)
	if block == "" {
		return nil, fmt.Errorf("%w: empty tool_calls block", ErrMalformedLegacyTool)
	}
	inner := block
	if strings.HasPrefix(block, tagToolCallsOpen) {
		if !strings.HasSuffix(block, tagToolCallsClose) {
			return nil, fmt.Errorf("%w: unclosed tool_calls block", ErrMalformedLegacyTool)
		}
		inner = strings.TrimSpace(block[len(tagToolCallsOpen) : len(block)-len(tagToolCallsClose)])
	}
	if inner == "" {
		return nil, fmt.Errorf("%w: tool_calls block has no tools", ErrMalformedLegacyTool)
	}
	var invs []Invocation
	rest := inner
	for {
		loc := reToolOpen.FindStringSubmatchIndex(rest)
		if loc == nil {
			if junk := strings.TrimSpace(rest); junk != "" {
				return nil, fmt.Errorf("%w: unexpected content outside tool tags: %s", ErrMalformedLegacyTool, legacyErrorSnippet(junk))
			}
			break
		}
		if loc[0] > 0 {
			if junk := strings.TrimSpace(rest[:loc[0]]); junk != "" {
				return nil, fmt.Errorf("%w: unexpected content outside tool tags: %s", ErrMalformedLegacyTool, legacyErrorSnippet(junk))
			}
		}
		name := rest[loc[2]:loc[3]]
		if name == "" {
			return nil, fmt.Errorf("%w: tool name is empty", ErrMalformedLegacyTool)
		}
		afterOpen := rest[loc[1]:]
		closeLoc := reToolClose.FindStringIndex(afterOpen)
		if closeLoc == nil {
			return nil, fmt.Errorf("%w: unclosed tool tag for %q", ErrMalformedLegacyTool, name)
		}
		toolBody := afterOpen[:closeLoc[0]]
		rest = afterOpen[closeLoc[1]:]
		inv, err := parseToolBody(name, toolBody)
		if err != nil {
			return nil, err
		}
		invs = append(invs, inv)
	}
	if len(invs) == 0 {
		return nil, fmt.Errorf("%w: tool_calls block has no tools", ErrMalformedLegacyTool)
	}
	return invs, nil
}

func parseToolBody(name, body string) (Invocation, error) {
	intent := ""
	if m := reIntent.FindStringSubmatch(body); len(m) >= 2 {
		intent = strings.TrimSpace(m[1])
	}
	argsM := reArgs.FindStringSubmatch(body)
	if len(argsM) < 2 {
		return Invocation{}, fmt.Errorf("%w: tool %q missing <args> JSON object", ErrMalformedLegacyTool, name)
	}
	argsStr := strings.TrimSpace(argsM[1])
	if !json.Valid([]byte(argsStr)) {
		return Invocation{}, fmt.Errorf("%w: tool %q args must be valid JSON", ErrMalformedLegacyTool, name)
	}
	var argsMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(argsStr), &argsMap); err != nil {
		return Invocation{}, fmt.Errorf("%w: tool %q args must be a JSON object", ErrMalformedLegacyTool, name)
	}
	if argsMap == nil {
		argsMap = map[string]json.RawMessage{}
	}
	if intent != "" {
		b, err := json.Marshal(intent)
		if err != nil {
			return Invocation{}, err
		}
		argsMap["intent"] = b
	}
	merged, err := json.Marshal(argsMap)
	if err != nil {
		return Invocation{}, err
	}
	return Invocation{Name: name, Args: merged}, nil
}

func StripLegacyToolBlocks(text string) string {
	for {
		open := strings.Index(text, tagToolCallsOpen)
		if open < 0 {
			break
		}
		closeRel := strings.Index(text[open:], tagToolCallsClose)
		if closeRel < 0 {
			return strings.TrimSpace(text[:open])
		}
		close := open + closeRel + len(tagToolCallsClose)
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
	}
	return strings.TrimSpace(text)
}

func LegacyProseOutsideToolCalls(text string) string {
	open := strings.Index(text, tagToolCallsOpen)
	if open < 0 {
		return strings.TrimSpace(text)
	}
	closeRel := strings.Index(text[open:], tagToolCallsClose)
	if closeRel < 0 {
		return strings.TrimSpace(text[:open])
	}
	close := open + closeRel + len(tagToolCallsClose)
	before := strings.TrimSpace(text[:open])
	after := strings.TrimSpace(text[close:])
	switch {
	case before != "" && after != "":
		return before + "\n" + after
	case before != "":
		return before
	default:
		return after
	}
}

func legacyErrorSnippet(s string) string {
	const max = 80
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) <= max {
		return strconv.Quote(s)
	}
	return strconv.Quote(s[:max] + "…")
}

func extractToolCallsBlock(text string) (string, bool, error) {
	open := strings.Index(text, tagToolCallsOpen)
	if open < 0 {
		return "", false, nil
	}
	close := strings.Index(text[open:], tagToolCallsClose)
	if close < 0 {
		return "", true, fmt.Errorf("%w: unclosed tool_calls block", ErrMalformedLegacyTool)
	}
	close += open
	return text[open : close+len(tagToolCallsClose)], false, nil
}
