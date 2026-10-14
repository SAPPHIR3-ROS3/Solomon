package test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logo"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func TestWelcomeLogoOverride(t *testing.T) {
	var output bytes.Buffer
	termcolor.Init(termcolor.InitOptions{Out: &output, ForceColor: true})
	t.Cleanup(func() { termcolor.Init(termcolor.InitOptions{Out: &output, NoColor: true}) })
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	wantDefault := logo.WelcomeLogoLines()
	if got := logo.WelcomeLogoLines(); !reflect.DeepEqual(got, wantDefault) {
		t.Fatal("missing override should use embedded crown")
	}
	for _, dir := range []string{"ascii", "colors"} {
		info, err := os.Stat(filepath.Join(home, "banner", dir))
		if err != nil || !info.IsDir() {
			t.Fatalf("banner directory %s not created: %v", dir, err)
		}
	}
	filePath := func(name string) string {
		dir := "ascii"
		if name == "colors.txt" {
			dir = "colors"
		}
		return filepath.Join(home, "banner", dir, name)
	}
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filePath(name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("logo.txt", " /\\ \r\n/王\\\r\n")
	got := logo.WelcomeLogoLines()
	if len(got) != 2 || got[0].Plain != " /\\" || got[1].Plain != "/王\\" {
		t.Fatalf("monochrome override not loaded: %#v", got)
	}
	write("colors.txt", "000000 FF0000 00FF00 000000\r\nFF0000 00FF00 0000FF\r\n")
	if colored := logo.WelcomeLogoLines(); len(colored) != 2 || colored[0].Plain != got[0].Plain || colored[0].ANSI == got[0].ANSI {
		t.Fatal("valid color map not loaded")
	}
	write("colors.txt", "FF0000 invalid\n")
	if got := logo.WelcomeLogoLines(); !reflect.DeepEqual(got, wantDefault) {
		t.Fatal("invalid color map should fall back to embedded crown")
	}
	if err := os.Remove(filePath("colors.txt")); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"", " \n⠀", "\xff", "\x1b[31mX", "X\tY", "X\rY", "X\u202eY", "X\u2028Y", strings.Repeat("X", 161), strings.Repeat("王", 81), strings.Repeat("X\n", 65), strings.Repeat("X", 64*1024+1)} {
		write("logo.txt", raw)
		if got := logo.WelcomeLogoLines(); !reflect.DeepEqual(got, wantDefault) {
			t.Fatalf("invalid logo should fall back (bytes=%d)", len(raw))
		}
	}
	write("logo.txt", strings.Repeat("王", 80))
	if got := logo.WelcomeLogoLines(); got[0].Plain != strings.Repeat("王", 80) {
		t.Fatal("160-cell logo should be accepted")
	}
	write("logo.txt", strings.Repeat("X\n", 64))
	if got := logo.WelcomeLogoLines(); len(got) != 64 {
		t.Fatal("64-row logo should be accepted")
	}
	if err := os.Remove(filePath("logo.txt")); err != nil {
		t.Fatal(err)
	}
	if got := logo.WelcomeLogoLines(); !reflect.DeepEqual(got, wantDefault) {
		t.Fatal("removing override should restore embedded crown")
	}
}
