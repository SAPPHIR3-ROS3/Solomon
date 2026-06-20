package termcolor

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

func ChatSeparatorWidth(termW int) int {
	if termW < 1 {
		return 24
	}
	n := (termW + 2) / 3
	if n < 1 {
		return 1
	}
	return n
}

func ChatSeparatorVisibleWidth(termW int) int {
	return ChatSeparatorWidth(termW)
}

func PrintChatSeparator(out io.Writer) {
	PrintChatSeparatorSized(out, terminalWidth(out))
}

func PrintChatSeparatorSized(out io.Writer, termW int) {
	n := ChatSeparatorWidth(termW)
	fmt.Fprint(out, WrapBoldGold(strings.Repeat("━", n))+"\n")
	flushOut(out)
}

func PrintBtwSeparator(out io.Writer) {
	PrintBtwSeparatorSized(out, terminalWidth(out))
}

func PrintBtwSeparatorSized(out io.Writer, termW int) {
	n := ChatSeparatorWidth(termW)
	fmt.Fprint(out, WrapContext(strings.Repeat("━", n))+"\n")
	flushOut(out)
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
	if err != nil || w < 20 {
		return 0
	}
	return w
}

func flushOut(out io.Writer) {
	if f, ok := out.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
}
