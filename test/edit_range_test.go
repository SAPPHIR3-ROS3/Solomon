package test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestEditFileLineRanges(t *testing.T) {
	cases := []struct {
		name, content, replacement, expected string
		bounds                               map[string]any
		old                                  string
	}{
		{"middle", "one\ntwo\nthree\nfour\n", "new", "one\nnew\nfour\n", map[string]any{"startLine": 2, "endLine": 3}, "two\nthree"},
		{"first", "one\ntwo\n", "new", "new\ntwo\n", map[string]any{"startLine": 1, "endLine": 1}, "one\n"},
		{"last", "one\ntwo", "new", "one\nnew", map[string]any{"startLine": 2, "endLine": 2}, "two"},
		{"last terminated", "one\ntwo\n", "new", "one\nnew\n", map[string]any{"startLine": 2, "endLine": 2}, "two"},
		{"delete", "one\ntwo\nthree\n", "", "one\nthree\n", map[string]any{"startLine": 2, "endLine": 2}, "two"},
		{"delete all", "one\ntwo\n", "", "", map[string]any{"startLine": 1, "endLine": 2}, ""},
		{"start only", "one\ntwo\nthree", "last", "one\nlast", map[string]any{"startLine": 2}, ""},
		{"end only", "one\ntwo\nthree\n", "first", "first\nthree\n", map[string]any{"endLine": 2}, ""},
		{"crlf", "one\r\ntwo\r\nthree\r\n", "new", "one\r\nnew\r\nthree\r\n", map[string]any{"startLine": 2, "endLine": 2}, "two"},
		{"crlf read guard", "one\r\ntwo\r\nthree\r\n", "new", "one\r\nnew\r\nthree\r\n", map[string]any{"startLine": 2, "endLine": 2}, "two\r"},
		{"crlf replacement from read", "one\r\ntwo\r\nthree\r\n", "new\r", "one\r\nnew\r\nthree\r\n", map[string]any{"startLine": 2, "endLine": 2}, "two\r"},
		{"unicode", "α\n日本語 😀\nω", "città", "α\ncittà\nω", map[string]any{"startLine": 2, "endLine": 2}, "日本語 😀"},
		{"blank line", "one\n\nthree\n", "new", "one\nnew\nthree\n", map[string]any{"startLine": 2, "endLine": 2}, ""},
		{"blank file", "\n", "new", "new\n", map[string]any{"startLine": 1, "endLine": 1}, ""},
		{"explicit ending", "one\ntwo\nthree\n", "new\n\n", "one\nnew\n\nthree\n", map[string]any{"startLine": 2, "endLine": 2}, ""},
		{"multiline", "one\ntwo\nthree", "a\nb", "one\na\nb\nthree", map[string]any{"startLine": 2, "endLine": 2}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "file.txt")
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			args := tc.bounds
			args["path"], args["intent"], args["newString"], args["oldString"] = "file.txt", "replace selected lines", tc.replacement, tc.old
			raw, err := json.Marshal(args)
			if err != nil {
				t.Fatal(err)
			}
			result, err := execEditFileForTest(t, dir, raw)
			if err != nil {
				t.Fatal(err)
			}
			if m := result.(map[string]any); m["ok"] != true || m["action"] != "edited" {
				t.Fatalf("unexpected result: %#v", result)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != tc.expected {
				t.Fatalf("got %q err=%v, want %q", got, err, tc.expected)
			}
		})
	}
}

func TestEditFileInvalidRangesDoNotMutate(t *testing.T) {
	cases := map[string]map[string]any{
		"zero":         {"startLine": 0, "endLine": 1, "newString": "new"},
		"negative":     {"startLine": -1, "newString": "new"},
		"reversed":     {"startLine": 2, "endLine": 1, "newString": "new"},
		"past end":     {"endLine": 4, "newString": "new"},
		"missing body": {"startLine": 1},
		"null body":    {"startLine": 1, "newString": nil},
		"null start":   {"startLine": nil, "newString": "new"},
		"null end":     {"endLine": nil, "newString": "new"},
		"delete":       {"startLine": 1, "delete": true, "newString": ""},
		"rename":       {"endLine": 2, "renameTo": "moved.txt", "newString": ""},
		"stale text":   {"startLine": 1, "oldString": "old version", "newString": "new"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "file.txt")
			content := "first\nsecond\n"
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			args["path"], args["intent"] = "file.txt", "reject invalid range"
			raw, err := json.Marshal(args)
			if err != nil {
				t.Fatal(err)
			}
			checkpoints := 0
			env := &tools.Env{ProjRoot: dir, AllowDeferredTools: true, CheckpointBeforeProjAbs: func(string) { checkpoints++ }}
			result, err := tools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: "editFile", Args: raw})
			if err == nil && result.(map[string]any)["ok"] != false {
				t.Fatalf("invalid edit succeeded: %#v", result)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != content || checkpoints != 0 {
				t.Fatalf("rejected edit mutated state: content=%q checkpoints=%d err=%v", got, checkpoints, err)
			}
		})
	}
}

func TestEditFileRangeDoesNotCreateFiles(t *testing.T) {
	dir := t.TempDir()
	_, err := execEditFileForTest(t, dir, json.RawMessage(`{"path":"missing.txt","startLine":1,"newString":"new","intent":"reject missing file"}`))
	if err == nil {
		t.Fatal("range edit created a missing file")
	}
	if _, err := os.Stat(filepath.Join(dir, "missing.txt")); !os.IsNotExist(err) {
		t.Fatalf("unexpected file: %v", err)
	}
}

func TestEditFileRangeSDKAndLegacyReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("first\nsecond\nthird\nlast\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := `package main
import ("fmt"; "sdk")
func main() {
 if err := sdk.EditFile("file.txt", "stale content", "bad", "reject stale range", false, sdk.EditRange{StartLine: 2, EndLine: 3}); err == nil { panic("stale edit succeeded") }
 result, err := sdk.EditFileResult("file.txt", "second\nthird", "new", "replace selected lines", false, sdk.EditRange{StartLine: 2, EndLine: 3})
 if err != nil || !result.OK { panic(fmt.Sprint(result, err)) }
 if err := sdk.ReplaceInFile("file.txt", "first", "FIRST", "replace exact text"); err != nil { panic(err) }
 if err := sdk.EditFile("file.txt", "last", "LAST", "legacy edit", false); err != nil { panic(err) }
 fmt.Printf("%d:%d", result.StartLine, result.EndLine)
}`
	args, err := json.Marshal(map[string]string{"source": source, "intent": "verify range and legacy SDK edits"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := tools.Exec(context.Background(), &tools.Env{ProjRoot: dir}, "agent", tooling.Invocation{Name: "orchestrate", Args: args})
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	if m["ok"] != false || m["script_ok"] != true || m["status"] != "partial" || m["failed_tool_calls"] != 1 || m["output"] != "2:3" {
		t.Fatalf("SDK edit failed: %#v", m)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "FIRST\nnew\nLAST\n" {
		t.Fatalf("unexpected SDK edit: %q err=%v", got, err)
	}
}

func TestEditFileRangeDisplayAndDiscovery(t *testing.T) {
	lines := tooling.FormatToolDisplayLines("editFile", json.RawMessage(`{"path":"file.go","startLine":2,"endLine":4,"newString":"new"}`))
	if len(lines) == 0 || !strings.Contains(termcolor.Plain(lines[0]), "file.go (lines 2-4)") {
		t.Fatalf("range not shown: %#v", lines)
	}
	result, err := tools.Exec(context.Background(), &tools.Env{}, "agent", tooling.Invocation{Name: "searchTools", Args: json.RawMessage(`{"query":"edit line range","intent":"discover line edits"}`)})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"EditRange", "startLine", "ReplaceInFile"} {
		if !strings.Contains(string(raw), expected) {
			t.Fatalf("discovery missing %s: %s", expected, raw)
		}
	}
}
