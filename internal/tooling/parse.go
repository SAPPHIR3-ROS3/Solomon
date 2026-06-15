package tooling

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrMalformedLegacyTool = errors.New("malformed legacy tool block")

var ErrUnknownLegacyTool = errors.New("unknown legacy tool name")

var ErrLegacyToolBlockComplete = errors.New("legacy tool_calls block complete")

type Invocation struct {
	Name       string
	Args       json.RawMessage
	ToolCallID string
}

func ValidateInvocationNames(invs []Invocation, allowed map[string]struct{}) error {
	if len(allowed) == 0 {
		return nil
	}
	for _, inv := range invs {
		if _, ok := allowed[inv.Name]; !ok {
			return fmt.Errorf("%w: %q", ErrUnknownLegacyTool, inv.Name)
		}
	}
	return nil
}

func ExtractToolInvocations(text string) ([]Invocation, error) {
	block, hasOpen, err := extractToolCallsBlock(text)
	if err != nil {
		return nil, err
	}
	if !hasOpen && block == "" {
		return nil, nil
	}
	return ParseToolCallsBlock(block)
}

func ValidateLegacyToolLines(text string) error {
	_, err := ExtractToolInvocations(text)
	return err
}

func UserFacingLegacyToolError(err error) string {
	if err == nil {
		return ""
	}
	for e := err; e != nil; e = errors.Unwrap(e) {
		if errors.Is(e, ErrUnknownLegacyTool) {
			return userFacingUnknownTool(e)
		}
	}
	malformedPrefix := ErrMalformedLegacyTool.Error() + ":"
	for e := err; e != nil; e = errors.Unwrap(e) {
		if strings.HasPrefix(e.Error(), malformedPrefix) {
			return userFacingMalformedTool(e)
		}
	}
	if errors.Is(err, ErrMalformedLegacyTool) {
		return userFacingMalformedTool(err)
	}
	return "Legacy tool syntax error: " + err.Error()
}

func userFacingUnknownTool(err error) string {
	msg := err.Error()
	const p = "unknown legacy tool name: "
	if i := strings.Index(msg, p); i >= 0 {
		return "Legacy tool syntax error: unknown tool " + strings.TrimSpace(msg[i+len(p):]) + " (not in ## Available tools for this mode)."
	}
	return "Legacy tool syntax error: unknown tool name (not available in this mode)."
}

func userFacingMalformedTool(err error) string {
	detail := strings.TrimPrefix(err.Error(), ErrMalformedLegacyTool.Error()+": ")
	detail = strings.TrimSpace(detail)
	switch {
	case detail == "empty tool_calls block":
		return "Legacy tool syntax error: empty <tool_calls> block."
	case detail == "unclosed tool_calls block":
		return "Legacy tool syntax error: <tool_calls> opened but </tool_calls> is missing."
	case detail == "tool_calls block has no tools":
		return "Legacy tool syntax error: <tool_calls> contains no <tool name=\"...\"> entries."
	case detail == "tool name is empty":
		return "Legacy tool syntax error: <tool name=\"\"> must be a non-empty tool name from ## Available tools."
	case strings.HasPrefix(detail, "unclosed tool tag for "):
		name := strings.Trim(strings.TrimPrefix(detail, "unclosed tool tag for "), `"`)
		return fmt.Sprintf("Legacy tool syntax error: tool %q is missing </tool>.", name)
	case strings.HasPrefix(detail, "tool ") && strings.Contains(detail, " missing <args>"):
		name := extractQuotedToolName(detail)
		return fmt.Sprintf("Legacy tool syntax error: tool %q requires <args>{...}</args> with a JSON object.", name)
	case strings.HasPrefix(detail, "tool ") && strings.Contains(detail, " args must be valid JSON"):
		name := extractQuotedToolName(detail)
		return fmt.Sprintf("Legacy tool syntax error: tool %q has invalid JSON inside <args>.", name)
	case strings.HasPrefix(detail, "tool ") && strings.Contains(detail, " args must be a JSON object"):
		name := extractQuotedToolName(detail)
		return fmt.Sprintf("Legacy tool syntax error: tool %q <args> must be a JSON object, not an array or string.", name)
	case strings.HasPrefix(detail, "unexpected content outside tool tags"):
		if junk := strings.TrimSpace(strings.TrimPrefix(detail, "unexpected content outside tool tags:")); junk != "" && detail != junk {
			return fmt.Sprintf("Legacy tool syntax error: only <tool> entries are allowed inside <tool_calls>; remove stray text %s.", junk)
		}
		return "Legacy tool syntax error: only <tool> entries are allowed inside <tool_calls> (no prose or extra tags between tools)."
	default:
		return "Legacy tool syntax error: " + detail + "."
	}
}

func extractQuotedToolName(detail string) string {
	i := strings.Index(detail, `"`)
	if i < 0 {
		return "?"
	}
	j := strings.Index(detail[i+1:], `"`)
	if j < 0 {
		return "?"
	}
	return detail[i+1 : i+1+j]
}
