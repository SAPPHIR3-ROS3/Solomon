package test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"
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
