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
	if width <= 0 {
		width = terminalWidth(out)
	}
	if width <= 0 {
		width = 120
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
			fmt.Fprint(out, "\n")
			return true, ctx.Err()
		case <-sigCh:
			fmt.Fprint(out, "\n")
			return true, nil
		case <-tick.C:
			filled++
			bar := termcolor.WrapBoldGold(strings.Repeat("─", filled)) + strings.Repeat(" ", width-filled)
			fmt.Fprintf(out, "\r%s", bar)
		}
		if time.Now().After(deadline) {
			break
		}
	}
	fmt.Fprint(out, "\n")
	r.Mode = target
	commands.PrintSystem(out, "Mode: "+target)
	return false, nil
}

func terminalWidth(out io.Writer) int {
	f, ok := out.(*os.File)
	if !ok {
		return 0
	}
	fd := int(f.Fd())
	if !term.IsTerminal(fd) {
		return 0
	}
	w, _, err := term.GetSize(fd)
	if err != nil || w < 1 {
		return 0
	}
	return w
}

func SwitchModeCountdownForTest(r *Runtime, ctx context.Context, target string) (cancelled bool, err error) {
	return r.switchModeCountdown(ctx, target)
}
