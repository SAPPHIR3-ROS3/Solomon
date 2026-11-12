package test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/research"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestOrchestrateReportsHostOutcomes(t *testing.T) {
	cases := []struct {
		name, body, status, kind string
		scriptOK                 bool
		failed, successful       int
	}{
		{"shell exit", `_, err := sdk.Shell("printf x >> marker; exit 7", "exercise shell exit"); fmt.Println(err); _, err = sdk.ReadFile("file.txt", "continue after shell failure"); if err != nil { panic(err) }`, "partial", "shell_exit", true, 1, 1},
		{"caught read error", `_, err := sdk.ReadFile("missing.txt", "exercise missing file"); fmt.Println(err); if err := sdk.WriteFile("fallback.txt", "fallback", "continue with fallback"); err != nil { panic(err) }`, "partial", "host_error", true, 1, 1},
		{"structured edit failure", `result, err := sdk.ReplaceInFileResult("file.txt", "absent", "new", "exercise rejected edit"); fmt.Println(result.OK, err)`, "partial", "tool_error", true, 1, 0},
		{"successful output contains error words", `value, err := sdk.ReadFile("file.txt", "read ordinary error-related text"); if err != nil { panic(err) }; fmt.Print(value)`, "success", "", true, 0, 1},
		{"script panic", `panic("script failure")`, "error", "", false, 0, 0},
		{"script and host failure", `_, err := sdk.ReadFile("missing.txt", "exercise missing file before panic"); if err != nil { panic(err) }`, "error", "host_error", false, 1, 0},
		{"bounded failure list", `for i := 0; i < 20; i++ { _, err := sdk.ReadFile("missing.txt", "exercise bounded failure reporting"); fmt.Println(err) }`, "partial", "host_error", true, 20, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("error failed panic are ordinary data"), 0o600); err != nil {
				t.Fatal(err)
			}
			source := "package main\nimport(\"fmt\";\"sdk\")\nfunc main(){_ = fmt.Print; _ = sdk.ReadFile; " + tc.body + "}"
			result := runOutcomeScript(t, &tools.Env{ProjRoot: dir}, source)
			if result["ok"] != (tc.status == "success") || result["script_ok"] != tc.scriptOK || result["status"] != tc.status || result["failed_tool_calls"] != tc.failed || result["successful_tool_calls"] != tc.successful {
				t.Fatalf("unexpected result: %#v", result)
			}
			if result["observed_tool_calls"] != tc.failed+tc.successful {
				t.Fatalf("incorrect call total: %#v", result)
			}
			if tc.failed > 0 {
				raw, err := json.Marshal(result["tool_errors"])
				if err != nil {
					t.Fatal(err)
				}
				var failures []struct {
					Call int    `json:"call"`
					Tool string `json:"tool"`
					Kind string `json:"kind"`
				}
				if err := json.Unmarshal(raw, &failures); err != nil {
					t.Fatal(err)
				}
				if len(failures) == 0 || len(failures) > 16 || failures[0].Call != 1 || failures[0].Kind != tc.kind {
					t.Fatalf("unexpected tool failures: %s", raw)
				}
				if tc.failed > 16 && result["tool_errors_truncated"] != true {
					t.Fatalf("failure truncation was hidden: %#v", result)
				}
			} else if result["tool_errors"] != nil {
				t.Fatalf("successful script has tool errors: %#v", result)
			}
			if tc.name == "shell exit" {
				marker, err := os.ReadFile(filepath.Join(dir, "marker"))
				if err != nil || string(marker) != "x" {
					t.Fatalf("shell side effects were repeated: %q %v", marker, err)
				}
			}
		})
	}
}

func TestOrchestrateReportsShellLaunchFailure(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing-directory")
	result := runOutcomeScript(t, &tools.Env{ProjRoot: dir}, `package main
import("fmt"; "sdk")
func main(){ _, err := sdk.Shell("printf hello", "exercise launch failure"); fmt.Print(err) }`)
	if result["ok"] != false || result["status"] != "partial" || result["failed_tool_calls"] != 1 || !strings.Contains(result["output"].(string), "shell exit -1") {
		t.Fatalf("shell launch failure was hidden: %#v", result)
	}
}

func TestOrchestrateOutcomeCountersArePerRun(t *testing.T) {
	dir := t.TempDir()
	failed := runOutcomeScript(t, &tools.Env{ProjRoot: dir}, `package main
import("fmt"; "sdk")
func main(){ _, err := sdk.ReadFile("missing.txt", "exercise per-run state"); fmt.Print(err) }`)
	if failed["status"] != "partial" {
		t.Fatalf("expected a partial run: %#v", failed)
	}
	success := runOutcomeScript(t, &tools.Env{ProjRoot: dir}, `package main
import "fmt"
func main(){ fmt.Print("success") }`)
	if success["ok"] != true || success["status"] != "success" || success["failed_tool_calls"] != 0 || success["tool_errors"] != nil {
		t.Fatalf("previous failures leaked into a later run: %#v", success)
	}
}

func runOutcomeScript(t *testing.T, env *tools.Env, source string) map[string]any {
	t.Helper()
	args, err := json.Marshal(map[string]string{"source": source, "intent": "verify orchestration outcome reporting"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := tools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: "orchestrate", Args: args})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := result.(map[string]any)
	if !ok || m["compile_error"] != nil {
		t.Fatalf("unexpected script result: %#v", result)
	}
	return m
}

func TestOrchestrateStructuredErrorIsNotASuccess(t *testing.T) {
	env := &tools.Env{ResearchStatus: func(id string) (research.JobRecord, error) {
		return research.JobRecord{ID: id, Status: research.StatusFailed, Error: "upstream failed"}, nil
	}}
	result := runOutcomeScript(t, env, `package main
import("fmt"; "sdk")
func main(){ value, err := sdk.ResearchStatus("job", "inspect failed research"); fmt.Println(value, err) }`)
	if result["ok"] != false || result["script_ok"] != true || result["status"] != "partial" || result["failed_tool_calls"] != 1 {
		t.Fatalf("structured error was hidden: %#v", result)
	}
}

func TestOrchestratePartialDisplayPrecedesScriptOutput(t *testing.T) {
	payload := `{"ok":false,"script_ok":true,"status":"partial","failed_tool_calls":1,"output":"Everything succeeded"}`
	lines := tooling.FormatToolResultDisplayLines("orchestrate", payload)
	text := termcolor.Plain(strings.Join(lines, "\n"))
	if !strings.Contains(text, "partial (1 failed sdk calls)") || strings.Contains(text, "Everything succeeded") {
		t.Fatalf("display hides partial failure: %q", text)
	}
}
