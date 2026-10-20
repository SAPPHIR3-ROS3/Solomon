package test

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
	"strings"
	"testing"
)

func TestOrchestrateParseDiagnostics(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"escape", "_ = \"\\d+\"", "invalid escape in a Go quoted literal"},
		{"raw", "_ = `unfinished", "close the raw string with a backtick"},
		{"quoted", "_ = \"unfinished", "close the double-quoted Go string"},
		{"comma", "_ = []string{\"one\"\n}", "check the separator"},
		{"illegal", "_ = $value", "this character is not valid Go syntax"},
		{"declaration", "}\n}", "only declarations are allowed outside functions"},
		{"parenthesis", "if (true { }", "check matching parentheses"},
		{"bracket", "var x [2 int", "check matching parentheses"},
		{"brace", "if true {", "check matching parentheses"},
		{"operand", "_ = )", "an expression is required here"},
		{"separator", "_ = 1 2", "check the preceding statement"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package main\nimport \"sdk\"\nfunc main() { " + tc.body + "\n}\n"
			_, err := compile.BuildWASM(compile.Options{Source: src})
			if err == nil {
				t.Fatal("expected syntax error")
			}
			for _, want := range []string{"invalid Go source", "Source near main.go:", tc.want, "SDK import"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("missing %q in %s", want, err)
				}
			}
			if strings.Contains(err.Error(), "a common cause") {
				t.Fatal("unexpected generic diagnosis")
			}
		})
	}
}
