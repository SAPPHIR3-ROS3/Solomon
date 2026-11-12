package compile

import (
	"go/parser"
	"go/scanner"
	"go/token"
	"strings"
)

type SourceDiagnostic struct {
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Message  string `json:"message"`
	Category string `json:"category,omitempty"`
	Hint     string `json:"hint,omitempty"`
}

func SourceDiagnostics(source string) []SourceDiagnostic {
	_, err := parser.ParseFile(token.NewFileSet(), "main.go", source, parser.AllErrors)
	list, ok := err.(scanner.ErrorList)
	if !ok {
		return nil
	}
	var diagnostics []SourceDiagnostic
	for _, entry := range list {
		diagnostic := SourceDiagnostic{
			Line: entry.Pos.Line, Column: entry.Pos.Column, Message: entry.Msg,
		}
		if strings.Contains(entry.Msg, "escape") || strings.Contains(entry.Msg, "string literal") || strings.Contains(entry.Msg, "illegal character") {
			diagnostic.Hint = "Move the affected payload into orchestrate.inputs and read it with sdk.Input(name); JSON-escape the input once, without Go escaping."
		}
		diagnostics = append(diagnostics, diagnostic)
		if len(diagnostics) == 8 {
			break
		}
	}
	return diagnostics
}
