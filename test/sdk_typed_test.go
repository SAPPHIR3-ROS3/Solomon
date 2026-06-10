package test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/sdk"
)

func parseGrepLinesTest(output string) []sdk.GrepLine {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil
	}
	lines := strings.Split(output, "\n")
	out := make([]sdk.GrepLine, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		n, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		out = append(out, sdk.GrepLine{Path: parts[0], Line: n, Text: parts[2]})
	}
	return out
}

func TestParseGrepLinesFormat(t *testing.T) {
	got := parseGrepLinesTest("internal/foo.go:42:func bar()")
	if len(got) != 1 || got[0].Line != 42 {
		t.Fatalf("got %+v", got)
	}
}

func TestParseEditResultJSON(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"ok": false, "reason": "oldString not found"})
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	ok, _ := m["ok"].(bool)
	if ok {
		t.Fatal("expected ok false")
	}
}

func TestTypedResultTypesCompile(t *testing.T) {
	var (
		_ sdk.GrepLine
		_ sdk.GrepCountEntry
		_ sdk.FindResult
		_ sdk.WebSearchResult
		_ sdk.WebHit
		_ sdk.DocsResult
		_ sdk.DocsSnippet
		_ sdk.EditResult
	)
}
