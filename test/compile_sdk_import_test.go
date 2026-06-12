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

func TestRewriteSDKImports_aliases(t *testing.T) {
	canon := compile.SDKImportCanonical
	cases := []struct {
		in   string
		want string
	}{
		{`package main
import "sdk"
func main() {}`, canon},
		{`package main
import "SAPPHIR3ROS3/Solomon/v2026/sdk"
func main() {}`, canon},
		{`package main
import (
	"fmt"
	"SAPPHIR3ROS3/Solomon/sdk"
)
func main() {}`, canon},
	}
	for _, tc := range cases {
		got := compile.RewriteSDKImports(tc.in)
		if !strings.Contains(got, `"`+canon+`"`) {
			t.Fatalf("rewrite failed for %q:\n%s", tc.in, got)
		}
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
	if !strings.Contains(s, "SAPPHIR3ROS3/Solomon/v2026/sdk") {
		t.Fatalf("missing model import alias: %s", s)
	}
}

func TestCompileSDKImportAlias(t *testing.T) {
	src := `package main

import (
	"fmt"
	"SAPPHIR3ROS3/Solomon/v2026/sdk"
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
