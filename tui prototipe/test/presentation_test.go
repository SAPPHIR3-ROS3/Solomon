package test

import (
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func presentation(t *testing.T, kind string) string {
	t.Helper()
	cmd := exec.Command("../atelier", "--snapshot", "120", "40", kind)
	cmd.Env = append(os.Environ(), "CLICOLOR_FORCE=1", "COLORTERM=truecolor", "TERM=xterm-256color")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v %s", kind, err, out)
	}
	return string(out)
}
func TestSearchToolsCardIsNotJSON(t *testing.T) {
	output := presentation(t, "search")
	for _, label := range []string{"searchTools", "tool matches", "Shell"} {
		if !strings.Contains(output, label) {
			t.Errorf("missing %s", label)
		}
	}
	for _, raw := range []string{"\"query\"", "\"intent\"", "\"sdk\"", "\"tools\"", "\"arguments\""} {
		if strings.Contains(output, raw) {
			t.Errorf("raw JSON leaked: %s", raw)
		}
	}
}
func TestGoHighlightsTokens(t *testing.T) {
	output := presentation(t, "code")
	color := regexp.MustCompile("\\x1b\\[[0-9;]+m")
	colors := map[string]bool{}
	for _, row := range strings.Split(output, "\n") {
		if !strings.Contains(row, "Preview source:") {
			continue
		}
		for _, match := range color.FindAllString(row, -1) {
			colors[match] = true
		}
	}
	if len(colors) < 4 {
		t.Errorf("expected token colors on a single source line, got %d", len(colors))
	}
}
func TestActualImagePixels(t *testing.T) {
	output := presentation(t, "image")
	if !strings.Contains(output, "actual image pixels") {
		t.Fatal("missing real image caption")
	}
	if strings.Count(output, "▀") < 100 {
		t.Fatal("PNG pixels were not rendered")
	}
	if !strings.Contains(output, "48;2;") {
		t.Fatal("missing lower-half pixel colors")
	}
	if strings.Contains(output, "[terminal preview]") || strings.Contains(output, "thumbnail placeholder") {
		t.Fatal("image placeholder returned")
	}
	if lipgloss.Width(output) > 120 || lipgloss.Height(output) > 40 {
		t.Fatal("image viewport overflow")
	}
}
func TestSlashSuggestions(t *testing.T) {
	output := presentation(t, "slash")
	for _, label := range []string{"/agent", "/artifacts", "/chat", "/clear", "/demo", "Tab complete", "Enter run"} {
		if !strings.Contains(output, label) {
			t.Errorf("missing slash suggestion %s", label)
		}
	}
}
func TestUserGoldFrame(t *testing.T) {
	output := presentation(t, "user")
	plain := regexp.MustCompile("\\x1b\\[[0-9;]*m").ReplaceAllString(output, "")
	rows := strings.Split(plain, "\n")
	start, end := -1, -1
	for i, row := range rows {
		if strings.Contains(row, "User messages keep") {
			start = i
		}
	}
	if start < 1 {
		t.Fatal("missing user content")
	}
	for i := 0; i < start; i++ {
		if strings.Contains(rows[i], "╭") {
			end = i
		}
	}
	if end < 0 || !strings.Contains(rows[start+1], "╰") {
		t.Fatal("user message is not enclosed by a frame")
	}
}
func TestArchiveHasNoMockupTruncation(t *testing.T) {
	data, err := os.ReadFile("../fixtures/editor-images.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"[line excerpt]", "remaining lines omitted", "[excerpt;"} {
		if strings.Contains(string(data), marker) {
			t.Errorf("irreversible truncation: %s", marker)
		}
	}
	var archive struct{ Entries []struct{ Role, Text string } }
	if err = json.Unmarshal(data, &archive); err != nil {
		t.Fatal(err)
	}
	longest := 0
	for _, entry := range archive.Entries {
		longest = max(longest, len(entry.Text))
	}
	if longest < 20000 {
		t.Fatalf("expected full long tool contents, longest is %d bytes", longest)
	}
}

func TestCompleteToolCallComponent(t *testing.T) {
	cmd := exec.Command("../laboratory", "--snapshot", "120", "100", "tool_call")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	plain := regexp.MustCompile("\\x1b\\[[0-9;]*m").ReplaceAllString(string(out), "")
	for _, label := range []string{"Tool: orchestrate", "Source", "func main()", "SDK calls", "Result"} {
		if !strings.Contains(plain, label) {
			t.Errorf("complete tool call missing %q", label)
		}
	}
	if strings.Contains(string(out), "[line excerpt]") {
		t.Fatal("complete card must not truncate the archived tool")
	}
}

func TestWrappedStringsKeepSyntaxColor(t *testing.T) {
	sgr := regexp.MustCompile("\\x1b\\[[0-9;]*m")
	for _, name := range []string{"atelier", "orbit", "paper", "mission"} {
		for _, width := range []string{"42", "80", "120"} {
			out, err := exec.Command("../"+name, "--snapshot", width, "80", "wrapped_strings").Output()
			if err != nil {
				t.Fatal(err)
			}
			checked := 0
			for _, row := range strings.Split(string(out), "\n") {
				target := "xxxxxxxx"
				index := strings.Index(row, target)
				if index < 0 {
					target = "RAW-SECOND"
					index = strings.Index(row, target)
				}
				if index < 0 {
					continue
				}
				active := ""
				for _, code := range sgr.FindAllString(row[:index], -1) {
					if code == "\x1b[0m" {
						active = ""
					} else if strings.Contains(code, "38;") {
						active = code
					}
				}
				if !strings.Contains(active, "38;2;227;148;220") {
					t.Errorf("%s width %s: continuation lost string color before %q", name, width, target)
				}
				checked++
			}
			if checked < 3 {
				t.Errorf("%s width %s: insufficient wrapped lines tested (%d)", name, width, checked)
			}
		}
	}
}

func TestWrappedCodeKeepsTwoSpaceIndentation(t *testing.T) {
	strip := regexp.MustCompile("\\x1b\\[[0-9;]*m")
	for _, name := range []string{"atelier", "orbit", "paper", "mission"} {
		out, err := exec.Command("../"+name, "--snapshot", "80", "100", "wrapped_strings").Output()
		if err != nil {
			t.Fatal(err)
		}
		continued := 0
		for _, row := range strings.Split(strip.ReplaceAllString(string(out), ""), "\n") {
			index := strings.Index(row, "xxxxxxxx")
			if index < 0 {
				continue
			}
			codeStart := strings.Index(row, "│ ")
			if name == "mission" {
				codeStart = strings.Index(row, "┃ ")
			}
			prefix := 0
			if codeStart >= 0 {
				prefix = codeStart + len("│ ")
				if name == "mission" {
					prefix = codeStart + len("┃ ")
				}
			} else {
				prefix = 3
			}
			content := row[prefix:]
			if strings.Contains(content, "value :=") {
				if !strings.Contains(content, "  value :=") {
					t.Errorf("%s: source indentation is not two spaces", name)
				}
				continue
			}
			if !strings.HasPrefix(content, "      x") {
				t.Errorf("%s: wrapped code is not aligned after gutter and two spaces: %q", name, content)
			}
			continued++
		}
		if continued < 2 {
			t.Errorf("%s: expected multiple continuations, got %d", name, continued)
		}
	}
}

func TestToolCallExecutionBeforeSource(t *testing.T) {
	out, err := exec.Command("../laboratory", "--snapshot", "120", "100", "tool_call").Output()
	if err != nil {
		t.Fatal(err)
	}
	plain := regexp.MustCompile("\\x1b\\[[0-9;]*m").ReplaceAllString(string(out), "")
	tool := strings.Index(plain, "Tool: orchestrate")
	execution := strings.Index(plain, "SDK calls")
	source := strings.Index(plain, "Source")
	result := strings.LastIndex(plain, "Result")
	if tool < 0 || execution <= tool || source <= execution || result <= source {
		t.Fatal("expected tool, SDK call metadata, source, then result")
	}
	if strings.Count(plain, "SDK calls") != 1 {
		t.Fatal("SDK call metadata duplicated")
	}
}
