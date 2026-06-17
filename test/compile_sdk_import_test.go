package test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestRewriteSDKImports_sdkOnly(t *testing.T) {
	canon := compile.SDKImportCanonical
	src := `package main
import "sdk"
func main() {}`
	got, err := compile.RewriteSDKImports(src)
	if err != nil {
		t.Fatalf("rewrite parse error: %v", err)
	}
	if !strings.Contains(got, `"`+canon+`"`) {
		t.Fatalf("rewrite failed:\n%s", got)
	}
}

func TestRewriteSDKImports_ignoresLegacyAliases(t *testing.T) {
	legacy := `package main
import "SAPPHIR3ROS3/Solomon/v2026/sdk"
func main() {}`
	got, err := compile.RewriteSDKImports(legacy)
	if err != nil {
		t.Fatalf("rewrite parse error: %v", err)
	}
	if strings.Contains(got, compile.SDKImportCanonical) {
		t.Fatalf("legacy alias should not be rewritten:\n%s", got)
	}
	if !strings.Contains(got, `SAPPHIR3ROS3/Solomon/v2026/sdk`) {
		t.Fatalf("legacy import path changed unexpectedly:\n%s", got)
	}
}

func TestSearchToolsSDKImportsOmitCanonicalPath(t *testing.T) {
	out, err := agenttools.Exec(context.Background(), &agenttools.Env{}, "agent", tooling.Invocation{
		Name: "searchTools",
		Args: json.RawMessage(`{"query":"sdk"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(out)
	s := string(raw)
	if strings.Contains(s, compile.SDKImportCanonical) {
		t.Fatalf("searchTools sdk ref should not expose canonical import path: %s", s)
	}
	if !strings.Contains(s, `"sdk"`) {
		t.Fatalf("missing sdk import alias: %s", s)
	}
	if strings.Contains(s, "SAPPHIR3ROS3/Solomon") {
		t.Fatalf("searchTools should not list legacy sdk import aliases: %s", s)
	}
	if !strings.Contains(s, "ReplaceInFile") {
		t.Fatalf("missing ReplaceInFile example: %s", s)
	}
	if !strings.Contains(s, "pitfalls") {
		t.Fatalf("missing orchestrate pitfalls: %s", s)
	}
}

func TestCompileSDKImportAlias(t *testing.T) {
	src := `package main

import (
	"fmt"
	"sdk"
)

func main() {
	_, err := sdk.ReadFile("README.md")
	if err != nil {
		fmt.Println("err")
	}
}
`
	if _, err := compile.BuildWASM(compile.Options{Source: src}); err != nil {
		t.Fatal(err)
	}
}

func TestBuildWASM_invalidSourceWithSDKImport(t *testing.T) {
	src := `package main

import (
	"fmt"
	"sdk"
)

func main() {
	content := ` + "`" + `# TODO
` + "`" + `webSearch` + "`" + `
` + "`" + `
	fmt.Print(content)
}
`
	_, err := compile.BuildWASM(compile.Options{Source: src})
	if err == nil {
		t.Fatal("expected compile error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "invalid Go source") {
		t.Fatalf("expected invalid Go source error, got: %s", msg)
	}
	if !strings.Contains(msg, "SDK import") {
		t.Fatalf("expected SDK import hint, got: %s", msg)
	}
}
