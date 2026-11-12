package test

import (
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/syntax"
)

func TestFormatGoPresentation(t *testing.T) {
	original := "package main;import(\"fmt\";\"sdk\");func main(){o,e:=sdk.ReadFile(\"file\",\"read\");if e!=nil{fmt.Println(e)};fmt.Println(o)}"
	formatted := syntax.FormatGo(original)
	for _, part := range []string{"func main() {", "\n  o, e :=", "\n  if e != nil {", "\n    fmt.Println(e)"} {
		if !strings.Contains(formatted, part) {
			t.Errorf("missing formatted fragment %q", part)
		}
	}
	if syntax.FormatGo(formatted) != formatted {
		t.Fatal("display formatting is not idempotent")
	}
	invalid := "func {"
	if syntax.FormatGo(invalid) != invalid {
		t.Fatal("invalid source must remain visible unchanged")
	}
}

func TestFormatGoPreservesLiteralContents(t *testing.T) {
	raw := "`first\n\tinside raw literal\nlast`"
	original := "package main;func main(){for i:=0;i<2;i++{println(" + raw + ")}}"
	formatted := syntax.FormatGo(original)
	if !strings.Contains(formatted, raw) {
		t.Fatal("raw literal content was modified")
	}
	if !strings.Contains(formatted, "for i := 0; i < 2; i++ {") {
		t.Fatal("for clause was split incorrectly")
	}
	if !strings.Contains(formatted, "\n    println(") {
		t.Fatal("nested indentation must be four spaces")
	}
	if strings.Contains(formatted, "\n\tfor") {
		t.Fatal("syntactic indentation still uses tabs")
	}
}
func TestSyntaxDisplayPolicyIsShared(t *testing.T) {
	if syntax.Display.IndentSize != 2 || syntax.Display.ContinuationIndent != "line" || syntax.Display.Ligatures {
		t.Fatal("unexpected syntax display policy")
	}
}
