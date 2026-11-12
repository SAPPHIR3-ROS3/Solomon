package syntax

import (
	_ "embed"
	"encoding/json"
)

type DisplayPolicy struct {
	IndentSize         int    `json:"indent_size"`
	ContinuationIndent string `json:"continuation_indent"`
	Ligatures          bool   `json:"ligatures"`
}

//go:embed display.json
var rawDisplay []byte

var Display = func() DisplayPolicy {
	var policy DisplayPolicy
	if err := json.Unmarshal(rawDisplay, &policy); err != nil {
		panic(err)
	}
	if policy.IndentSize < 1 || policy.IndentSize > 8 || policy.ContinuationIndent != "line" {
		panic("invalid syntax display policy")
	}
	return policy
}()
