package termcolor

import (
	"io"
	"os"
	"regexp"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

var (
	initMu    sync.Mutex
	colorOn   bool
	ansiStrip = regexp.MustCompile(`\x1b\[[0-9;:]*m`)
)

type InitOptions struct {
	Out        io.Writer
	NoColor    bool
	ForceColor bool
}

func Init(opts InitOptions) {
	initMu.Lock()
	defer initMu.Unlock()
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	colorOn = colorsEnabled(out, opts.NoColor, opts.ForceColor)
	r := lipgloss.NewRenderer(out)
	if !colorOn {
		r.SetColorProfile(termenv.Ascii)
	} else {
		r.SetColorProfile(termenv.ANSI256)
	}
	lipgloss.SetDefaultRenderer(r)
	rebuildTheme()
}

func Enabled() bool {
	return colorOn
}

func Plain(s string) string {
	return ansiStrip.ReplaceAllString(s, "")
}

func colorsEnabled(w io.Writer, explicitNoColor, forceColor bool) bool {
	if explicitNoColor {
		return false
	}
	if forceColor {
		return true
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("CLICOLOR") == "0" {
		return false
	}
	if !writerIsTerminal(w) {
		return false
	}
	return true
}

func writerIsTerminal(w io.Writer) bool {
	type fdWriter interface {
		Fd() uintptr
	}
	f, ok := w.(fdWriter)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func profile() termenv.Profile {
	if !colorOn {
		return termenv.Ascii
	}
	return lipgloss.DefaultRenderer().ColorProfile()
}

func renderStyle(st lipgloss.Style, s string) string {
	if !colorOn {
		return s
	}
	return st.Render(s)
}
