package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestFormatToolDisplayLines_editFileLargePatchHeadTail(t *testing.T) {
	body := strings.Repeat("line\n", 120)
	args, _ := json.Marshal(map[string]string{
		"path":      "big.go",
		"oldString": "",
		"newString": body,
		"intent":    "create module",
	})
	lines := tooling.FormatToolDisplayLines("editFile", args)
	if len(lines) != 22 {
		t.Fatalf("want header + 10 head + truncated + 10 tail, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "big.go") {
		t.Fatalf("want path in header: %q", lines[0])
	}
	if lines[11] == "" || !strings.Contains(lines[11], "TRUNCATED") {
		t.Fatalf("want truncated marker at index 11, got %q", lines[11])
	}
	if strings.Count(strings.Join(lines, "\n"), "line") != 20 {
		t.Fatalf("want exactly 20 content lines shown, got %q", strings.Join(lines, "\n"))
	}
}

func TestFormatToolDisplayLines_editFileDiffStripsCommonLines(t *testing.T) {
	args, _ := json.Marshal(map[string]string{
		"path":      "agent.tmpl",
		"oldString": "same\nold-only\nshared-tail\n",
		"newString": "same\nnew-only\nshared-tail\n",
	})
	lines := tooling.FormatToolDisplayLines("editFile", args)
	if len(lines) != 3 {
		t.Fatalf("want header + removed + added, got %d: %#v", len(lines), lines)
	}
	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, "same") || strings.Contains(joined, "shared-tail") {
		t.Fatalf("common lines should be omitted: %q", joined)
	}
	if !strings.Contains(joined, "old-only") || !strings.Contains(joined, "new-only") {
		t.Fatalf("want only differing lines: %q", joined)
	}
}

func TestFormatToolDisplayLines_editFileSplitsBeforeWrap(t *testing.T) {
	args, _ := json.Marshal(map[string]string{
		"path":      "x.go",
		"oldString": "line1\nline2\n",
		"newString": "line3\n",
	})
	lines := tooling.FormatToolDisplayLines("editFile", args)
	if len(lines) != 4 {
		t.Fatalf("want header + 2 old + 1 new lines, got %d: %#v", len(lines), lines)
	}
}

func TestFormatToolDisplayLines_editFileSkipsEmptyOldBlock(t *testing.T) {
	args, _ := json.Marshal(map[string]string{
		"path":      "x.go",
		"oldString": "",
		"newString": "package main\n",
	})
	lines := tooling.FormatToolDisplayLines("editFile", args)
	if len(lines) != 2 {
		t.Fatalf("want header + new only, got %d: %#v", len(lines), lines)
	}
}

func TestFormatToolDisplayLines_editFileSkipsEmptyNewBlock(t *testing.T) {
	args, _ := json.Marshal(map[string]string{
		"path":      "x.go",
		"oldString": "before",
		"newString": "",
	})
	lines := tooling.FormatToolDisplayLines("editFile", args)
	if len(lines) != 2 {
		t.Fatalf("want header + old only, got %d: %#v", len(lines), lines)
	}
}

func TestFormatToolDisplayLines_switchMode(t *testing.T) {
	args, err := json.Marshal(map[string]string{"mode": "agent"})
	if err != nil {
		t.Fatal(err)
	}
	lines := tooling.FormatToolDisplayLines("switchMode", args)
	if len(lines) != 1 {
		t.Fatalf("lines: %#v", lines)
	}
	plain := termcolor.Plain(lines[0])
	if !strings.Contains(plain, "Tool: switchMode Agent") {
		t.Fatalf("got %q", plain)
	}
	if strings.Contains(plain, "{") {
		t.Fatalf("should not show JSON args: %q", plain)
	}
}

func TestFormatToolDisplayLines_orchestrate(t *testing.T) {
	src := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"test\")\n}\n"
	args, err := json.Marshal(map[string]string{
		"source": src,
		"intent": "count characters",
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := tooling.FormatToolDisplayLines("orchestrate", args)
	if len(lines) != 9 {
		t.Fatalf("want header + 7 source + footer, got %d: %#v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "orchestrate") || !strings.Contains(lines[0], "Code") {
		t.Fatalf("header: %q", lines[0])
	}
	if !strings.Contains(termcolor.Plain(lines[1]), "1 package main") {
		t.Fatalf("first code line: %q", lines[1])
	}
	if !strings.Contains(termcolor.Plain(lines[6]), "6") || !strings.Contains(termcolor.Plain(lines[6]), "fmt.Println") {
		t.Fatalf("indented line: %q", lines[6])
	}
	if termcolor.Plain(lines[8]) != "Code" {
		t.Fatalf("footer: %q", lines[8])
	}
}

func TestFormatToolDisplayLines_orchestrateExpandsTabs(t *testing.T) {
	src := "func main() {\n\tfmt.Println(\"x\")\n}\n"
	args, err := json.Marshal(map[string]string{"source": src, "intent": "tab test"})
	if err != nil {
		t.Fatal(err)
	}
	lines := tooling.FormatToolDisplayLines("orchestrate", args)
	plain := termcolor.Plain(lines[2])
	if strings.Contains(plain, "\t") {
		t.Fatalf("display should not contain tab chars: %q", plain)
	}
	if !strings.Contains(plain, "    fmt.Println") {
		t.Fatalf("tab should expand to 4 spaces: %q", plain)
	}
}

func TestFormatToolDisplayLines_orchestrateTruncatesLongSource(t *testing.T) {
	body := strings.Repeat("fmt.Println(\"x\")\n", 60)
	src := "package main\n\nfunc main() {\n" + body + "}\n"
	args, err := json.Marshal(map[string]string{"source": src, "intent": "stress test"})
	if err != nil {
		t.Fatal(err)
	}
	lines := tooling.FormatToolDisplayLines("orchestrate", args)
	if len(lines) != 53 {
		t.Fatalf("want header + 25 + truncated + 25 + footer = 53, got %d", len(lines))
	}
	if !strings.Contains(lines[26], "TRUNCATED") {
		t.Fatalf("truncated marker: %q", lines[26])
	}
}

func TestFormatToolDisplayLines_searchTools(t *testing.T) {
	args, err := json.Marshal(map[string]any{"query": "edit file"})
	if err != nil {
		t.Fatal(err)
	}
	lines := tooling.FormatToolDisplayLines("searchTools", args)
	if len(lines) != 1 {
		t.Fatalf("lines: %#v", lines)
	}
	if !strings.Contains(lines[0], "searchTools edit file") {
		t.Fatalf("got %q", lines[0])
	}
	if strings.Contains(lines[0], "{") {
		t.Fatalf("should not show JSON args: %q", lines[0])
	}
}

func TestFormatToolDisplayLines_webSearch(t *testing.T) {
	args, err := json.Marshal(map[string]any{
		"query":          "golang context",
		"engine":         "duckduckgo",
		"maxResults":     5,
		"timeoutSeconds": 45,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := tooling.FormatToolDisplayLines("webSearch", args)
	if len(lines) != 1 {
		t.Fatalf("lines: %#v", lines)
	}
	want := "webSearch • duckduckgo (5 result • 45s) | golang context"
	if !strings.Contains(lines[0], want) {
		t.Fatalf("got %q, want substring %q", lines[0], want)
	}

	minimal, err := json.Marshal(map[string]any{"query": "test"})
	if err != nil {
		t.Fatal(err)
	}
	lines = tooling.FormatToolDisplayLines("webSearch", minimal)
	wantMinimal := "webSearch • | test"
	if !strings.Contains(lines[0], wantMinimal) {
		t.Fatalf("defaults: got %q, want substring %q", lines[0], wantMinimal)
	}

	onlyTimeout, err := json.Marshal(map[string]any{"query": "slow", "timeoutSeconds": 60})
	if err != nil {
		t.Fatal(err)
	}
	lines = tooling.FormatToolDisplayLines("webSearch", onlyTimeout)
	if !strings.Contains(lines[0], "webSearch • (60s) | slow") {
		t.Fatalf("custom timeout only: %q", lines[0])
	}

	onlyMax, err := json.Marshal(map[string]any{"query": "wide", "maxResults": 3})
	if err != nil {
		t.Fatal(err)
	}
	lines = tooling.FormatToolDisplayLines("webSearch", onlyMax)
	if !strings.Contains(lines[0], "webSearch • (3 result) | wide") {
		t.Fatalf("custom maxResults only: %q", lines[0])
	}
}

func TestFormatToolDisplayLines_editFileDelete(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout, ForceColor: true})
	args, err := json.Marshal(map[string]any{
		"path":   "obsolete.go",
		"delete": true,
		"intent": "remove dead code",
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := tooling.FormatToolDisplayLines("editFile", args)
	if len(lines) != 1 {
		t.Fatalf("lines: %#v", lines)
	}
	if termcolor.Plain(lines[0]) != "Tool: editFile obsolete.go" {
		t.Fatalf("unexpected delete display plain: %q", termcolor.Plain(lines[0]))
	}
	if !strings.Contains(lines[0], termcolor.WrapRed("obsolete.go")) {
		t.Fatalf("delete path should be red: %q", lines[0])
	}
}

func TestFormatToolDisplayLines_deleteRemoveRedArg(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout, ForceColor: true})
	delArgs, _ := json.Marshal(map[string]string{"name": "old.md"})
	lines := tooling.FormatToolDisplayLines("deletePlan", delArgs)
	if !strings.Contains(lines[0], termcolor.WrapRed("old.md")) {
		t.Fatalf("deletePlan arg should be red: %q", lines[0])
	}
	rmArgs, _ := json.Marshal(map[string]string{"sha1": "abc123deadbeef"})
	lines = tooling.FormatToolDisplayLines("removeTodo", rmArgs)
	if !strings.Contains(lines[0], termcolor.WrapRed("abc123deadbeef")) {
		t.Fatalf("removeTodo arg should be red: %q", lines[0])
	}
}

func TestFormatToolResultDisplayLines_editFileSuccessSilent(t *testing.T) {
	for _, payload := range []string{
		`{"ok":true,"action":"edited"}`,
		`{"ok":true,"action":"deleted"}`,
		`{"ok":true,"action":"created_or_overwrite"}`,
	} {
		lines := tooling.FormatToolResultDisplayLines("editFile", payload)
		if len(lines) != 0 {
			t.Fatalf("payload %s: want no result line, got %v", payload, lines)
		}
	}
}

func TestFormatToolResultDisplayLines_addTodoSuccessSilent(t *testing.T) {
	payload := `{"ok":true,"sha":"89f93794061db4a04d4ba0f9c36915d8c073f7ad","status":"not_built"}`
	lines := tooling.FormatToolResultDisplayLines("addTodo", payload)
	if len(lines) != 0 {
		t.Fatalf("want no addTodo result line, got %v", lines)
	}
}

func TestFormatToolResultDisplayLines_addTodoFailureShowsReason(t *testing.T) {
	payload := `{"error":"todo is required"}`
	lines := tooling.FormatToolResultDisplayLines("addTodo", payload)
	if len(lines) != 1 || !strings.Contains(lines[0], "todo is required") {
		t.Fatalf("lines: %v", lines)
	}
}

func TestFormatToolResultDisplayLines_todoListSilent(t *testing.T) {
	payload := `{"abc123":"Do work","def456":"Other"}`
	lines := tooling.FormatToolResultDisplayLines("todoList", payload)
	if len(lines) != 0 {
		t.Fatalf("want no todoList result line, got %v", lines)
	}
}

func TestFormatToolResultDisplayLines_editFileFailureShowsReason(t *testing.T) {
	payload := `{"ok":false,"reason":"oldString not found"}`
	lines := tooling.FormatToolResultDisplayLines("editFile", payload)
	if len(lines) != 1 || !strings.Contains(lines[0], "oldString not found") {
		t.Fatalf("lines: %v", lines)
	}
}

func TestFormatToolResultDisplayLines_readFileOmitsContent(t *testing.T) {
	payload := `{"path":"TODO.md","total_lines":141,"content":"# TODO\n\nlong body"}`
	lines := tooling.FormatToolResultDisplayLines("readFile", payload)
	if len(lines) != 1 {
		t.Fatalf("lines: %v", lines)
	}
	if strings.Contains(lines[0], "# TODO") || strings.Contains(lines[0], "long body") {
		t.Fatalf("must not echo file body: %q", lines[0])
	}
	if !strings.Contains(lines[0], "TODO.md") || !strings.Contains(lines[0], "141") {
		t.Fatalf("want path and line count: %q", lines[0])
	}
}

func TestFormatToolDisplayLines_planTools(t *testing.T) {
	createArgs, _ := json.Marshal(map[string]string{"name": "feat.md", "goal": "Add planning"})
	lines := tooling.FormatToolDisplayLines("createPlan", createArgs)
	if len(lines) != 1 || !strings.Contains(termcolor.Plain(lines[0]), "Tool: createPlan feat.md") {
		t.Fatalf("createPlan: %#v", lines)
	}

	editArgs, _ := json.Marshal(map[string]string{
		"name": "feat.md", "old": "old-only\n", "new": "new-only\n", "intent": "update design",
	})
	lines = tooling.FormatToolDisplayLines("editPlan", editArgs)
	if len(lines) != 3 {
		t.Fatalf("editPlan want header+2 diff lines, got %d: %#v", len(lines), lines)
	}
	if !strings.Contains(termcolor.Plain(lines[0]), "Tool: editPlan feat.md") {
		t.Fatalf("editPlan header: %q", lines[0])
	}

	buildArgs, _ := json.Marshal(map[string]string{"name": "feat.md"})
	lines = tooling.FormatToolDisplayLines("buildPlan", buildArgs)
	if !strings.Contains(termcolor.Plain(lines[0]), "Tool: buildPlan feat.md") {
		t.Fatalf("buildPlan: %#v", lines)
	}

	todoArgs, _ := json.Marshal(map[string]string{"name": "feat.md", "todo": "Write tests"})
	lines = tooling.FormatToolDisplayLines("addTodo", todoArgs)
	if !strings.Contains(termcolor.Plain(lines[0]), "Tool: addTodo feat.md") {
		t.Fatalf("addTodo: %#v", lines)
	}
}

func TestFormatToolDisplayLines_orchestrateNestedParensColored(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout, ForceColor: true})
	src := "func f(a (int)) { x[(y)] }\n"
	args, _ := json.Marshal(map[string]string{"source": src, "intent": "paren colors"})
	lines := tooling.FormatToolDisplayLines("orchestrate", args)
	if len(lines) < 2 {
		t.Fatal("expected orchestrate lines")
	}
	code := lines[1]
	if !strings.Contains(code, "\x1b[") {
		t.Fatalf("expected ANSI highlights: %q", code)
	}
	if strings.Count(code, "\x1b[") < 4 {
		t.Fatalf("expected multiple color spans for nested parens: %q", code)
	}
}

func TestFormatToolDisplayLines_subagentDefaultTemplate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	tplDir := filepath.Join(home, "prompts", "templates")
	if err := os.MkdirAll(tplDir, 0o700); err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(tplDir, "agent.tmpl")
	args, _ := json.Marshal(map[string]string{
		"sysPromptPath": abs,
		"task":          "Rispondi solamente OK",
	})
	lines := tooling.FormatToolDisplayLines("subagent", args)
	if len(lines) < 2 {
		t.Fatalf("want at least 2 lines, got %d", len(lines))
	}
	plain0 := termcolor.Plain(lines[0])
	if plain0 != "Tool: subagent agent (sync)" {
		t.Fatalf("header %q", plain0)
	}
	if strings.Contains(plain0, ".tmpl") || strings.Contains(plain0, home) {
		t.Fatalf("header should be template name only: %q", plain0)
	}
	plain1 := termcolor.Plain(lines[1])
	if strings.Contains(plain1, "Tool: subagent") {
		t.Fatalf("task line should not repeat tool header: %q", plain1)
	}
	if plain1 != "Rispondi solamente OK" {
		t.Fatalf("task %q", plain1)
	}
}

func TestFormatToolDisplayLines_subagentCustomPath(t *testing.T) {
	args, _ := json.Marshal(map[string]string{
		"sysPromptPath": "/tmp/custom-sys.txt",
		"task":          "do thing",
	})
	lines := tooling.FormatToolDisplayLines("subagent", args)
	plain0 := termcolor.Plain(lines[0])
	if !strings.Contains(plain0, "/tmp/custom-sys.txt") {
		t.Fatalf("custom path unchanged: %q", plain0)
	}
}
