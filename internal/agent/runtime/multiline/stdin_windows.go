//go:build windows

package multiline

import (
	"io"
	"os"
	"syscall"
	"unsafe"

	readline "github.com/chzyer/readline"
)

const (
	enableMouseInput    = 0x0010
	enableQuickEditMode = 0x0040
	enableExtendedFlags = 0x0080
	enableVTInput       = 0x0200
)

type nopCloseStdin struct{ io.Reader }

func (nopCloseStdin) Close() error { return nil }

func platformStdin() stdinReadCloser {
	if os.Getenv("WT_SESSION") != "" {
		return nopCloseStdin{Reader: os.Stdin}
	}
	return readline.NewRawReader()
}

func PrepareConsoleInput() func() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getMode := kernel32.NewProc("GetConsoleMode")
	setMode := kernel32.NewProc("SetConsoleMode")
	handle := syscall.Handle(os.Stdin.Fd())
	var mode uint32
	r, _, e := syscall.Syscall(getMode.Addr(), 2, uintptr(handle), uintptr(unsafe.Pointer(&mode)), 0)
	if r == 0 || e != 0 {
		return func() {}
	}
	old := mode
	mode &^= enableMouseInput
	if os.Getenv("WT_SESSION") != "" {
		mode |= enableExtendedFlags | enableVTInput
		mode &^= enableQuickEditMode
	}
	_, _, _ = syscall.Syscall(setMode.Addr(), 2, uintptr(handle), uintptr(mode), 0)
	return func() {
		_, _, _ = syscall.Syscall(setMode.Addr(), 2, uintptr(handle), uintptr(old), 0)
	}
}
