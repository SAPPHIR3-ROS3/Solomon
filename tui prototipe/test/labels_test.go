package test

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

func TestNoPreviewLabels(t *testing.T) {
	forbidden := regexp.MustCompile("(?i)\\bpreview\\b")
	for _, name := range []string{"atelier", "orbit", "paper", "mission"} {
		for _, state := range []string{"home", "slash", "model", "reasoning", "palette"} {
			out, err := exec.Command("../"+name, "--snapshot", "120", "40", state).CombinedOutput()
			if err != nil {
				t.Fatalf("%s/%s: %v", name, state, err)
			}
			if forbidden.Match(out) {
				t.Errorf("%s/%s contains a preview label", name, state)
			}
		}
	}
}

func TestMockupChromeIsSeparateFromChat(t *testing.T) {
	blue := regexp.MustCompile("\\x1b\\[[0-9;]*48;2;6;28;59[0-9;]*m")
	strip := regexp.MustCompile("\\x1b\\[[0-9;]*m")
	for _, name := range []string{"atelier", "orbit", "paper", "mission"} {
		cmd := exec.Command("../"+name, "--snapshot", "120", "40", "user")
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		plain := strip.ReplaceAllString(string(out), "")
		if strings.Contains(plain, "Agent · code mode") || strings.Contains(plain, "artifacts ·") {
			t.Errorf("%s still shows redundant labels", name)
		}
		rows := strings.Split(string(out), "\n")
		if !blue.MatchString(rows[len(rows)-1]) {
			t.Errorf("%s: missing Solomon-blue mockup toolbar", name)
		}
		for i, row := range rows[:len(rows)-1] {
			if blue.MatchString(row) {
				t.Errorf("%s: blue leaked into chat/composer row %d", name, i)
			}
		}
	}
}

func TestGalleryHasNoIllustrativeLabels(t *testing.T) {
	out, err := exec.Command("../laboratory", "--transcript", "gallery").Output()
	if err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile("(?i)illustrative").Match(out) {
		t.Fatal("illustrative label is still visible")
	}
}

func TestEmptyComposerCursorBeforePlaceholder(t *testing.T) {
	strip := regexp.MustCompile("\\x1b\\[[0-9;]*m")
	for _, name := range []string{"atelier", "orbit", "paper", "mission"} {
		out, err := exec.Command("../"+name, "--snapshot", "120", "40").Output()
		if err != nil {
			t.Fatal(err)
		}
		plain := strip.ReplaceAllString(string(out), "")
		placeholder := "Ask anything, or /help"
		if name == "mission" {
			placeholder = "Ask Solomon anything..."
		}
		if !strings.Contains(plain, "▏"+placeholder) {
			t.Errorf("%s: cursor must precede empty placeholder", name)
		}
		out, err = exec.Command("../"+name, "--snapshot", "120", "40", "slash").Output()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strip.ReplaceAllString(string(out), ""), "/▏") {
			t.Errorf("%s: cursor must follow typed draft", name)
		}
	}
}
