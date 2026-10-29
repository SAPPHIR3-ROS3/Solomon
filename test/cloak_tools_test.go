package test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/cloak"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/run"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/sdk"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestCloakToolsDiscovery(t *testing.T) {
	for _, query := range []string{"cloak", "CloakBrowser", "browser snapshot"} {
		t.Run(query, func(t *testing.T) {
			args, err := json.Marshal(map[string]string{"query": query, "intent": "Discover native browser tools"})
			if err != nil {
				t.Fatal(err)
			}
			out, err := agenttools.Exec(t.Context(), &agenttools.Env{}, "agent", tooling.Invocation{Name: "searchTools", Args: args})
			if err != nil {
				t.Fatal(err)
			}
			entries := out.(map[string]any)["tools"].([]map[string]string)
			for _, entry := range entries {
				if entry["name"] != "cloakBrowser" {
					continue
				}
				if entry["origin_mode"] != "deferred" || entry["sdk_call"] != "CloakBrowser(tool string, args map[string]any, intent string) (map[string]any, error)" {
					t.Fatalf("unexpected entry: %#v", entry)
				}
				var got []cloak.ToolSpec
				if err := json.Unmarshal([]byte(entry["supported_tools"]), &got); err != nil {
					t.Fatal(err)
				}
				expected, err := json.Marshal(cloak.Tools())
				if err != nil {
					t.Fatal(err)
				}
				actual, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				if len(got) == 0 || string(actual) != string(expected) {
					t.Fatal("discovery does not match the native catalog")
				}
				var schema map[string]any
				if err := json.Unmarshal([]byte(entry["parameters"]), &schema); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(schema["required"], []any{"tool", "args", "intent"}) {
					t.Fatalf("missing required fields: %#v", schema)
				}
				return
			}
			t.Fatal("CloakBrowser missing from discovery")
		})
	}
	found := false
	for _, name := range agenttools.AgentDeferredToolNames() {
		if name == "cloakBrowser" {
			found = true
		}
	}
	if !found {
		t.Fatal("CloakBrowser missing from deferred tool names")
	}
}

func TestCloakToolsGatingAndValidation(t *testing.T) {
	for _, tc := range []struct {
		name, mode, raw, want string
		deferred              bool
		client                *cloak.Client
	}{
		{name: "direct agent", mode: "agent", raw: "{\"tool\":\"unknown\",\"intent\":\"Inspect page\"}", want: "not available"},
		{name: "direct chat", mode: "chat", raw: "{\"tool\":\"unknown\",\"intent\":\"Inspect page\"}", want: "not available"},
		{name: "deferred chat", mode: "chat", deferred: true, raw: "{\"tool\":\"unknown\",\"intent\":\"Inspect page\"}", want: "not available"},
		{name: "missing intent", mode: "agent", deferred: true, raw: "{\"tool\":\"unknown\"}", want: "intent"},
		{name: "blank intent", mode: "agent", deferred: true, raw: "{\"tool\":\"unknown\",\"intent\":\"  \"}", want: "intent"},
		{name: "missing tool", mode: "agent", deferred: true, raw: "{\"intent\":\"Inspect page\"}", want: "tool is required"},
		{name: "bad args", mode: "agent", deferred: true, raw: "{\"tool\":\"unknown\",\"args\":[],\"intent\":\"Inspect page\"}", want: "cannot unmarshal"},
		{name: "unavailable client", mode: "agent", deferred: true, raw: "{\"tool\":\"unknown\",\"intent\":\"Inspect page\"}", want: "client unavailable"},
		{name: "native dispatch", mode: "agent", deferred: true, client: &cloak.Client{}, raw: "{\"tool\":\"unknown\",\"args\":{},\"intent\":\"Inspect page\"}", want: "unknown Cloak tool"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := agenttools.Exec(t.Context(), &agenttools.Env{AllowDeferredTools: tc.deferred, Cloak: tc.client}, tc.mode, tooling.Invocation{Name: "cloakBrowser", Args: json.RawMessage(tc.raw)})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
	if _, err := sdk.CloakBrowser("unknown", nil, " \t"); err == nil || !strings.Contains(err.Error(), "intent") {
		t.Fatalf("SDK accepted empty intent: %v", err)
	}
}

type cloakSDKCaller struct{ calls int }

func (c *cloakSDKCaller) Call(ctx context.Context, name string, raw json.RawMessage) (json.RawMessage, error) {
	c.calls++
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	expected := map[string]any{"tool": "browser_test", "args": map[string]any{"tabId": "tab-7", "intent": "remote argument"}, "intent": "Inspect browser page"}
	if name != "cloakBrowser" || !reflect.DeepEqual(args, expected) {
		return nil, fmt.Errorf("unexpected call %s: %#v", name, args)
	}
	return json.Marshal(map[string]any{"tabId": "tab-7", "nested": map[string]any{"ok": true}})
}

func TestCloakToolsSDKWASM(t *testing.T) {
	source := "package main\nimport (\"fmt\"; \"sdk\")\nfunc main() {\n args := map[string]any{\"tabId\": \"tab-7\", \"intent\": \"remote argument\"}\n result, err := sdk.CloakBrowser(\"browser_test\", args, \"Inspect browser page\")\n if err != nil { panic(err) }\n fmt.Print(result[\"tabId\"], \":\", result[\"nested\"].(map[string]any)[\"ok\"])\n}\n"
	wasm, err := compile.BuildWASM(compile.Options{Source: source})
	if err != nil {
		t.Fatal(err)
	}
	caller := &cloakSDKCaller{}
	runner, err := run.NewRunner(t.Context(), caller, run.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close(t.Context())
	out, err := runner.Run(t.Context(), wasm)
	if err != nil {
		t.Fatal(err)
	}
	if out != "tab-7:true" || caller.calls != 1 {
		t.Fatalf("output=%q calls=%d", out, caller.calls)
	}
}

func TestCloakUploadRejectsOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "private.txt")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{outside, "missing.txt"} {
		raw, err := json.Marshal(map[string]any{"tool": "upload_file", "args": map[string]any{"pageId": "t1", "uid": "u1", "filePaths": []string{path}}, "intent": "verify upload boundary"})
		if err != nil {
			t.Fatal(err)
		}
		_, err = agenttools.Exec(t.Context(), &agenttools.Env{ProjRoot: root, Cloak: &cloak.Client{}, AllowDeferredTools: true}, "agent", tooling.Invocation{Name: "cloakBrowser", Args: raw})
		if err == nil {
			t.Fatal("unsafe upload accepted")
		}
		if path == outside && !strings.Contains(err.Error(), "inside the workspace") {
			t.Fatal(err)
		}
	}
	link := filepath.Join(root, "escape.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("symlink creation is unavailable")
	}
	raw, err := json.Marshal(map[string]any{"tool": "upload_file", "args": map[string]any{"pageId": "t1", "uid": "u1", "filePaths": []string{link}}, "intent": "verify symlink boundary"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = agenttools.Exec(t.Context(), &agenttools.Env{ProjRoot: root, Cloak: &cloak.Client{}, AllowDeferredTools: true}, "agent", tooling.Invocation{Name: "cloakBrowser", Args: raw})
	if err == nil || !strings.Contains(err.Error(), "inside the workspace") {
		t.Fatalf("symlink accepted: %v", err)
	}
}
