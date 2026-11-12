package test

import (
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/syntax"
)

func TestSharedSyntaxPalette(t *testing.T) {
	cases := map[string]string{
		"keyword.function.go":            "#82D2CE",
		"string.quoted.double.go":        "#e394dc",
		"entity.name.function.go":        "#efb080",
		"constant.numeric.decimal.go":    "#ebc88d",
		"keyword.operator.assignment.go": "#efb080",
		"variable.other.go":              "#d6d6dd",
	}
	for scope, expected := range cases {
		if !strings.EqualFold(syntax.Color(scope), expected) {
			t.Errorf("%s: %s instead of %s", scope, syntax.Color(scope), expected)
		}
	}
	for index, expected := range []string{"#ffd700", "#ff00ff", "#87cefa", "#ffd700"} {
		if syntax.Bracket(index) != expected {
			t.Fatal("GUI rainbow palette mismatch")
		}
	}
	if !strings.Contains(syntax.ANSI("text", "#e394dc"), "38;2;227;148;220") {
		t.Fatal("syntax color was reduced to terminal ANSI palette")
	}
}
