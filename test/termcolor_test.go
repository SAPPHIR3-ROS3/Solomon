package test

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func TestTermcolorInitNonTTYPlain(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR", "")
	termcolor.Init(termcolor.InitOptions{Out: &bytes.Buffer{}})
	if termcolor.Enabled() {
		t.Fatal("expected colors disabled on non-TTY writer")
	}
	if got := termcolor.WrapUser("x"); got != "x" {
		t.Fatalf("WrapUser: got %q", got)
	}
}

func TestTermcolorInitNoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("CLICOLOR", "")
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout})
	if termcolor.Enabled() {
		t.Fatal("expected NO_COLOR to disable colors")
	}
}

func TestTermcolorInitCLICOLOR0(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR", "0")
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout})
	if termcolor.Enabled() {
		t.Fatal("expected CLICOLOR=0 to disable colors")
	}
}

func TestTermcolorInitExplicitNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR", "")
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout, NoColor: true})
	if termcolor.Enabled() {
		t.Fatal("expected explicit NoColor to disable colors")
	}
}

func TestTermcolorPlainStripsANSI(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR", "")
	termcolor.Init(termcolor.InitOptions{Out: &bytes.Buffer{}})
	styled := termcolor.WrapRed("err")
	if styled == "err" {
		t.Skip("no ANSI without TTY; test Plain on synthetic input")
	}
	plain := termcolor.Plain(styled)
	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("Plain left escapes: %q", plain)
	}
	if plain != "err" {
		t.Fatalf("Plain: got %q", plain)
	}
}

func TestTermcolorPlainSynthetic(t *testing.T) {
	in := "\x1b[91mhello\x1b[0m"
	out := termcolor.Plain(in)
	if out != "hello" {
		t.Fatalf("Plain: got %q", out)
	}
}

func TestTermcolorColorizeImgTagsPlainWhenDisabled(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: &bytes.Buffer{}, NoColor: true})
	got := termcolor.ColorizeImgTags("see [img-1] here")
	want := "see [img-1] here"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatSystemBlockPlain(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: &bytes.Buffer{}, NoColor: true})
	got := termcolor.FormatSystemBlock("loaded chat abc")
	want := "===SYSTEM===\nloaded chat abc\n===SYSTEM===\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatSystemBlockMultilineBordersUnpadded(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: &bytes.Buffer{}, NoColor: true})
	got := termcolor.FormatSystemBlock("short\n4\tgpt-4o[ChatGPT Sub]\n5\tgpt-4o-mini with a very long model id")
	for _, line := range strings.Split(strings.TrimSuffix(got, "\n"), "\n") {
		if strings.TrimSpace(line) == "===SYSTEM===" && line != "===SYSTEM===" {
			t.Fatalf("border line padded: %q", line)
		}
	}
}

func TestSystemMessageTextFromJSON(t *testing.T) {
	got := termcolor.SystemMessageText(`{"error":"timeout","code":42}`)
	if strings.Contains(got, "{") || strings.Contains(got, "}") {
		t.Fatalf("expected plain text, got %q", got)
	}
	if !strings.Contains(got, "error: timeout") || !strings.Contains(got, "code: 42") {
		t.Fatalf("unexpected plain text: %q", got)
	}
}

func TestWrapUserReadlinePlainWhenDisabled(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: &bytes.Buffer{}, NoColor: true})
	if got := termcolor.WrapUserReadline("You: "); got != "You: " {
		t.Fatalf("WrapUserReadline: got %q", got)
	}
}

func TestWrapUserReadlineWhenEnabled(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout, ForceColor: true})
	want := termcolor.WrapUser("You: ")
	if runtime.GOOS == "windows" {
		termcolor.SetREPLRawStdout(true)
		defer termcolor.SetREPLRawStdout(false)
	}
	if got := termcolor.WrapUserReadline("You: "); got != want {
		t.Fatalf("WrapUserReadline: got %q want %q", got, want)
	}
	if runtime.GOOS == "windows" {
		termcolor.SetREPLRawStdout(false)
		got := termcolor.WrapUserReadline("You: ")
		want := "\x1b[36mYou: \x1b[0m"
		if got != want {
			t.Fatalf("WrapUserReadline fallback: got %q want %q", got, want)
		}
	}
}

func TestColorizeErrorLines(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: &bytes.Buffer{}, NoColor: true})
	got := termcolor.ColorizeErrorLines("ok\n[error] boom\n  [error] spaced")
	want := "ok\n[error] boom\n  [error] spaced"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestErrorLineWriterStreamsPlainText(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: &bytes.Buffer{}, NoColor: true})
	var out bytes.Buffer
	w := termcolor.NewErrorLineWriter(&out)
	if _, err := w.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestErrorLineWriterRecognizesSplitMarker(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: &bytes.Buffer{}, NoColor: true})
	var out bytes.Buffer
	w := termcolor.NewErrorLineWriter(&out)
	if _, err := w.Write([]byte("[err")); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected buffered marker prefix, got %q", out.String())
	}
	if _, err := w.Write([]byte("or] boom")); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "[error] boom" {
		t.Fatalf("got %q", got)
	}
}
