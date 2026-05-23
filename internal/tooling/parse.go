package tooling

import (
	"encoding/json"
	"errors"
	"fmt"
)

var ErrMalformedLegacyTool = errors.New("malformed legacy tool block")

var ErrUnknownLegacyTool = errors.New("unknown legacy tool name")

var ErrLegacyToolBlockComplete = errors.New("legacy tool_calls block complete")

type Invocation struct {
	Name string
	Args json.RawMessage
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
