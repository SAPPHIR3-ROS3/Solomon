package test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/search"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/validation"
)

func TestInvalidArgumentsAreRejectedBeforeSideEffects(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
	}{
		{"shell", map[string]any{"command": "printf executed > marker", "timeoutSeconds": -1}},
		{"shell", map[string]any{"command": "printf executed > marker", "timeoutSeconds": "1"}},
		{"shell", map[string]any{"command": "   "}},
		{"readFile", map[string]any{"path": ""}},
		{"readFile", map[string]any{"path": "file.txt", "startLine": 2, "endLine": 1}},
		{"editFile", map[string]any{"path": "file.txt", "newString": nil}},
		{"webSearch", map[string]any{"query": "test", "maxResults": 0}},
		{"webSearch", map[string]any{"query": "test", "extras": []string{"bad"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			activated := false
			tc.args["intent"] = "reject invalid arguments"
			raw, err := json.Marshal(tc.args)
			if err != nil {
				t.Fatal(err)
			}
			env := &tools.Env{ProjRoot: dir, AllowDeferredTools: true, ActivateInstructionsFromShellCommand: func(string) { activated = true }, ActivateInstructionsFromAbsPath: func(string) { activated = true }}
			_, err = tools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: tc.name, Args: raw})
			if validation.Kind(err) != "arguments" || activated {
				t.Fatalf("invalid arguments reached execution: err=%v activated=%v", err, activated)
			}
			if _, err := os.Stat(filepath.Join(dir, "marker")); !os.IsNotExist(err) {
				t.Fatalf("command executed: %v", err)
			}
		})
	}
}

func TestMalformedArgumentsAndIntentAreTyped(t *testing.T) {
	for _, raw := range []string{`{`, `[]`, `null`, `{"command":"true"}`, `{"command":"true","intent":null}`} {
		_, err := tools.Exec(context.Background(), &tools.Env{}, "agent", tooling.Invocation{Name: "shell", Args: json.RawMessage(raw)})
		if validation.Kind(err) != "arguments" {
			t.Fatalf("unclassified arguments %q: %v", raw, err)
		}
	}
	_, err := tools.Exec(context.Background(), &tools.Env{}, "agent", tooling.Invocation{Name: "shell", Args: json.RawMessage(`{"command":"true"}`)})
	if !errors.Is(err, tooling.ErrMissingToolIntent) {
		t.Fatalf("missing intent cause was lost: %v", err)
	}
}

func TestSearchConfigurationIsValidatedWithoutSecrets(t *testing.T) {
	for _, tc := range []struct {
		engine string
		extras map[string]any
		kind   string
	}{
		{"brave", nil, "configuration"},
		{"bing", map[string]any{"apiKey": ""}, "configuration"},
		{"googlepse", map[string]any{"apiKey": "SECRET"}, "configuration"},
		{"brave", map[string]any{"apiKey": 123}, "arguments"},
		{"searxng", map[string]any{"baseURL": "ftp://SECRET@example.com"}, "configuration"},
	} {
		err := search.ValidateConfiguration(tc.engine, tc.extras)
		if validation.Kind(err) != tc.kind || strings.Contains(err.Error(), "SECRET") {
			t.Fatalf("bad configuration classification: %v", err)
		}
	}
	cfg := &config.Root{WebSearchAPIKey: "configured"}
	merged := tools.MergeWebSearchExtras(cfg, "brave", nil)
	if err := search.ValidateConfiguration("brave", merged); err != nil {
		t.Fatalf("configured provider rejected: %v", err)
	}
}

func TestSearchMissingCredentialsAreClassifiedThroughOrchestrate(t *testing.T) {
	source := `package main
import("fmt";"sdk")
func main(){_,err:=sdk.WebSearchEngineN("query","brave",1,"exercise missing credentials");fmt.Println(err)}`
	result := runOutcomeScript(t, &tools.Env{Cfg: config.EmptyRoot()}, source)
	if result["status"] != "partial" || result["failed_tool_calls"] != 1 {
		t.Fatalf("configuration failure hidden: %#v", result)
	}
	raw, err := json.Marshal(result["tool_errors"])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"category":"configuration"`) {
		t.Fatalf("configuration failure misclassified: %s", raw)
	}
}

func TestShellPolicyCannotBeBypassedWithLegacyDispatch(t *testing.T) {
	dir := t.TempDir()
	raw := json.RawMessage(`{"command":"printf executed > marker; node -e 'console.log(1)'","intent":"reject indirect inline execution"}`)
	_, err := tools.Exec(context.Background(), &tools.Env{ProjRoot: dir, AllowDeferredTools: true}, "agent", tooling.Invocation{Name: "shell", Args: raw})
	if validation.Kind(err) != "policy" {
		t.Fatalf("legacy dispatch bypassed shell policy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "marker")); !os.IsNotExist(err) {
		t.Fatalf("rejected command executed: %v", err)
	}
}
