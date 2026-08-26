package agentruntime

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"golang.org/x/term"
)

var (
	switchModeCountdownDuration = 5 * time.Second
	switchModeCountdownWidth    int
)

func SetSwitchModeCountdownForTest(d time.Duration, width int) {
	switchModeCountdownDuration = d
	switchModeCountdownWidth = width
}

func ResetSwitchModeCountdownForTest() {
	switchModeCountdownDuration = 5 * time.Second
	switchModeCountdownWidth = 0
}

func (r *Runtime) switchModeCountdown(ctx context.Context, target string) (cancelled bool, err error) {
	out := r.Out
	if out == nil {
		out = os.Stdout
	}
	fmt.Fprintln(out, termcolor.WrapSystem("Press Ctrl+C to cancel mode switch"))
	width := switchModeCountdownWidth
	terminalW := terminalWidth(out)
	interactive := terminalW > 0
	if width <= 0 {
		width = terminalW
	}
	if width <= 0 {
		width = 120
	}
	// Avoid an automatic terminal wrap at the rightmost cell. A wrapped
	// progress frame cannot be reliably replaced by the next frame.
	if interactive && width > 1 {
		width--
	}
	duration := switchModeCountdownDuration
	step := duration / time.Duration(width)
	if step <= 0 {
		step = time.Millisecond
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGINT)
	defer signal.Stop(sigCh)
	tick := time.NewTicker(step)
	defer tick.Stop()
	filled := 0
	deadline := time.Now().Add(duration)
	for filled < width {
		select {
		case <-ctx.Done():
			finishSwitchModeProgress(out)
			return true, ctx.Err()
		case <-sigCh:
			finishSwitchModeProgress(out)
			return true, nil
		case <-tick.C:
			filled++
			if interactive {
				bar := termcolor.WrapBoldGold(strings.Repeat("─", filled)) + strings.Repeat(" ", width-filled)
				writeSwitchModeProgress(out, bar)
			}
		}
		if time.Now().After(deadline) {
			break
		}
	}
	finishSwitchModeProgress(out)
	r.Mode = target
	commands.PrintSystem(out, "Mode: "+target)
	return false, nil
}

const switchModeClearLine = "\r\x1b[2K"

func writeSwitchModeProgress(out io.Writer, bar string) {
	fmt.Fprintf(out, "%s%s", switchModeClearLine, bar)
	flushWriter(out)
}

func finishSwitchModeProgress(out io.Writer) {
	// Keep the last (full or partial) frame visible and commit it with a
	// single newline instead of clearing it from the transcript.
	fmt.Fprintln(out)
	flushWriter(out)
}

func terminalWidth(out io.Writer) int {
	fd, ok := terminalFD(out)
	if !ok || !term.IsTerminal(fd) {
		return 0
	}
	w, _, err := term.GetSize(fd)
	if err != nil || w < 1 {
		return 0
	}
	return w
}

func terminalFD(out io.Writer) (int, bool) {
	if out == nil {
		return 0, false
	}
	if f, ok := out.(interface{ TerminalFD() (uintptr, bool) }); ok {
		fd, ok := f.TerminalFD()
		return int(fd), ok
	}
	if f, ok := out.(interface{ Fd() uintptr }); ok {
		return int(f.Fd()), true
	}
	return 0, false
}

func SwitchModeCountdownForTest(r *Runtime, ctx context.Context, target string) (cancelled bool, err error) {
	return r.switchModeCountdown(ctx, target)
}
