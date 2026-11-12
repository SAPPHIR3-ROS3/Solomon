package test

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestSnapshots(t *testing.T) {
	for _, name := range []string{"atelier", "orbit", "paper", "mission"} {
		for _, size := range [][2]int{{42, 24}, {80, 24}, {100, 32}, {160, 48}, {220, 60}} {
			for _, state := range []string{"home", "demo", "stream", "gallery", "palette", "slash", "image", "search", "code", "user"} {
				t.Run(fmt.Sprintf("%s/%dx%d/%s", name, size[0], size[1], state), func(t *testing.T) {
					cmd := exec.Command("../"+name, "--snapshot", fmt.Sprint(size[0]), fmt.Sprint(size[1]), state)
					cmd.Env = append(os.Environ(), "CLICOLOR_FORCE=1", "COLORTERM=truecolor", "TERM=xterm-256color")
					out, err := cmd.CombinedOutput()
					if err != nil {
						t.Fatalf("snapshot failed: %v %s", err, out)
					}
					if lipgloss.Width(string(out)) > size[0] {
						t.Errorf("width overflow: %d", lipgloss.Width(string(out)))
					}
					if lipgloss.Height(string(out)) > size[1] {
						t.Errorf("height overflow: %d", lipgloss.Height(string(out)))
					}
					if hasUnexpectedBackground(out) && state != "image" {
						t.Error("explicit background color detected")
					}
					labels := []string{"Ask anything", "F1 Atelier", "╭", "╰"}
					if name == "mission" {
						labels[0] = "Ask Solomon anything"
					}
					if state == "slash" {
						labels = []string{"/agent", "/artifacts", "Tab complete"}
					}
					if state == "palette" {
						labels = []string{"COMMAND CENTER", "New thread"}
					}
					for _, label := range labels {
						if !strings.Contains(string(out), label) {
							t.Errorf("missing %q", label)
						}
					}
					if state == "home" && !strings.Contains(string(out), "new thread") {
						t.Error("missing new thread banner context")
					}
					if state == "home" && strings.Contains(string(out), "Read 34 lines") {
						t.Error("new thread contains demo messages")
					}
				})
			}
		}
	}
}

func TestLayoutsAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range []string{"atelier", "orbit", "paper", "mission"} {
		out, err := exec.Command("../"+name, "--snapshot", "120", "40").Output()
		if err != nil {
			t.Fatal(err)
		}
		if seen[string(out)] {
			t.Fatalf("duplicate layout: %s", name)
		}
		seen[string(out)] = true
	}
}

func TestSmallTerminal(t *testing.T) {
	out, err := exec.Command("../atelier", "--snapshot", "30", "10").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "Resize") {
		t.Fatal("missing resize guidance")
	}
}

func TestOriginalBanner(t *testing.T) {
	raw, err := os.ReadFile("../../internal/logo/logo.txt")
	if err != nil {
		t.Fatal(err)
	}
	escape := regexp.MustCompile("\\x1b\\[[0-9;]*m")
	for _, name := range []string{"atelier", "orbit", "paper", "mission"} {
		for _, size := range [][2]int{{42, 24}, {100, 32}, {160, 48}} {
			out, err := exec.Command("../"+name, "--snapshot", fmt.Sprint(size[0]), fmt.Sprint(size[1])).Output()
			if err != nil {
				t.Fatal(err)
			}
			plain := escape.ReplaceAllString(string(out), "")
			for rowIndex, row := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
				if name == "mission" && rowIndex == 5 {
					continue
				}
				row = strings.TrimRight(row, " \u2800")
				if !strings.Contains(plain, row) {
					t.Errorf("%s %v: original banner row missing: %s", name, size, row)
				}
			}
		}
	}
}

func TestComposerPinnedToBottom(t *testing.T) {
	escape := regexp.MustCompile("\\x1b\\[[0-9;]*m")
	for _, name := range []string{"atelier", "orbit", "paper", "mission"} {
		for _, size := range [][2]int{{42, 24}, {80, 24}, {100, 32}, {160, 48}, {220, 60}} {
			for _, state := range []string{"demo", "stream", "gallery"} {
				out, err := exec.Command("../"+name, "--snapshot", fmt.Sprint(size[0]), fmt.Sprint(size[1]), state).Output()
				if err != nil {
					t.Fatal(err)
				}
				lines := strings.Split(escape.ReplaceAllString(string(out), ""), "\n")
				if len(lines) != size[1] {
					t.Fatalf("%s %v %s: got %d rows", name, size, state, len(lines))
				}
				if !strings.Contains(lines[size[1]-3], "╰") {
					t.Errorf("%s %v %s: composer is not pinned to the bottom", name, size, state)
				}
				if !strings.Contains(lines[size[1]-2], "enter send") {
					t.Errorf("%s %v %s: metadata is not directly below composer", name, size, state)
				}
				if state == "demo" && size[0] >= 80 && !strings.Contains(string(out), "━") && !strings.Contains(string(out), "─") {
					t.Errorf("%s: missing Solomon gold separator", name)
				}
			}
		}
	}
}

func TestFrameGeometry(t *testing.T) {
	escape := regexp.MustCompile("\\x1b\\[[0-9;]*m")
	for _, name := range []string{"atelier", "orbit", "paper", "mission"} {
		for _, size := range [][2]int{{42, 24}, {80, 24}, {120, 40}, {180, 60}} {
			out, err := exec.Command("../"+name, "--snapshot", fmt.Sprint(size[0]), fmt.Sprint(size[1])).Output()
			if err != nil {
				t.Fatal(err)
			}
			rows := strings.Split(escape.ReplaceAllString(string(out), ""), "\n")
			frameRight := -1
			frameBottom := -1
			composerTop := -1
			composerRight := -1
			for i, row := range rows {
				trimmed := strings.TrimLeft(row, " ")
				end := lipgloss.Width(strings.TrimRight(row, " "))
				if strings.HasPrefix(trimmed, "┌") {
					frameRight = end
					if size[0] >= 80 && !strings.Contains(row, "┬") {
						t.Errorf("%s: missing top column junction", name)
					}
				}
				if strings.HasPrefix(trimmed, "│") && frameBottom < 0 && frameRight > 0 && end != frameRight {
					t.Errorf("%s %v row %d: banner border ends at %d, want %d", name, size, i, end, frameRight)
				}
				if strings.HasPrefix(trimmed, "└") {
					frameBottom = i
					if end != frameRight {
						t.Errorf("%s: banner bottom is misaligned", name)
					}
				}
				if strings.HasPrefix(trimmed, "╭") {
					composerTop = i
					composerRight = end
				}
			}
			if name != "mission" && (frameBottom < 0 || composerTop != frameBottom+1) {
				t.Errorf("%s %v: composer must directly follow banner", name, size)
			}
			if name != "mission" && composerRight != frameRight {
				t.Errorf("%s %v: composer right edge %d differs from banner %d", name, size, composerRight, frameRight)
			}
			demo, err := exec.Command("../"+name, "--snapshot", fmt.Sprint(size[0]), fmt.Sprint(size[1]), "demo").Output()
			if err != nil {
				t.Fatal(err)
			}
			demoRows := strings.Split(escape.ReplaceAllString(string(demo), ""), "\n")
			end := lipgloss.Width(strings.TrimRight(demoRows[size[1]-3], " "))
			if end != size[0]-1 {
				t.Errorf("%s %v: composer ends at %d, want %d", name, size, end, size[0]-1)
			}
		}
	}
}

func TestLongArchivedTranscript(t *testing.T) {
	out, err := exec.Command("../atelier", "--transcript").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(out), "\n") < 350 {
		t.Fatal("archived mockup is too short")
	}
	for _, label := range []string{"03fcff4b9b2fc7e1625ea382b21fb1ea002a4aa496151239de749889659deb01", "vorrei che le immagini", "orchestrate", "Result", "func main()", "126 test", "c40e653"} {
		if !strings.Contains(regexp.MustCompile("\\x1b\\[[0-9;]*m").ReplaceAllString(string(out), ""), label) {
			t.Errorf("missing real archive artifact %q", label)
		}
	}
	gallery, err := exec.Command("../atelier", "--transcript", "gallery").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"Artifact gallery", "Subagent", "Checkpoint", "Permission request", "Image attachment", "Markdown", "Usage", "Tool failure", "/btw"} {
		if !strings.Contains(string(gallery), label) {
			t.Errorf("missing gallery artifact %q", label)
		}
	}
}

func hasUnexpectedBackground(data []byte) bool {
	codes := regexp.MustCompile("\\x1b\\[[0-9;]*m").FindAll(data, -1)
	for _, code := range codes {
		parameters := strings.Split(string(code[2:len(code)-1]), ";")
		for index := 0; index < len(parameters); index++ {
			value, _ := strconv.Atoi(parameters[index])
			if value == 38 || value == 48 {
				if index+1 >= len(parameters) {
					continue
				}
				mode, _ := strconv.Atoi(parameters[index+1])
				if mode == 2 && index+4 < len(parameters) {
					if value == 48 && (parameters[index+2] != "6" || parameters[index+3] != "28" || parameters[index+4] != "59") {
						return true
					}
					index += 4
				} else if mode == 5 && index+2 < len(parameters) {
					if value == 48 {
						return true
					}
					index += 2
				}
			} else if value >= 40 && value <= 47 || value >= 100 && value <= 107 {
				return true
			}
		}
	}
	return false
}
