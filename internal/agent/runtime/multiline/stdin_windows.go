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
	enableProcessedInput            = 0x0001
	enableLineInput                 = 0x0002
	enableEchoInput                 = 0x0004
	enableMouseInput                = 0x0010
	enableQuickEditMode             = 0x0040
	enableExtendedFlags             = 0x0080
	enableVTInput                   = 0x0200
	enableVirtualTerminalProcessing = 0x0004
)

var editorRawStdout bool

type nopCloseStdin struct{ io.Reader }

func (nopCloseStdin) Close() error { return nil }

func platformStdin() stdinReadCloser {
	if os.Getenv("WT_SESSION") != "" {
		return nopCloseStdin{Reader: os.Stdin}
	}
	return readline.NewRawReader()
}

func PrepareConsoleOutput() func() {
	editorRawStdout = false
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getMode := kernel32.NewProc("GetConsoleMode")
	setMode := kernel32.NewProc("SetConsoleMode")
	handle := syscall.Handle(os.Stdout.Fd())
	var mode uint32
	r, _, e := syscall.Syscall(getMode.Addr(), 2, uintptr(handle), uintptr(unsafe.Pointer(&mode)), 0)
	if r == 0 || e != 0 {
		return func() {}
	}
	old := mode
	if mode&enableVirtualTerminalProcessing != 0 {
		editorRawStdout = true
		return func() { editorRawStdout = false }
	}
	mode |= enableVirtualTerminalProcessing
	_, _, _ = syscall.Syscall(setMode.Addr(), 2, uintptr(handle), uintptr(mode), 0)
	editorRawStdout = true
	return func() {
		editorRawStdout = false
		_, _, _ = syscall.Syscall(setMode.Addr(), 2, uintptr(handle), uintptr(old), 0)
	}
}

func EditorUsesRawStdout() bool {
	return editorRawStdout
}

func EditorStdout(fallback io.Writer) io.Writer {
	if editorRawStdout {
		return os.Stdout
	}
	if fallback == nil {
		return os.Stdout
	}
	return fallback
}

func EnsureCookedTTY() {
	EnsureCookedFD(int(os.Stdin.Fd()))
}

func FlushStdin() {}

func EnterRawStdin() (restore func(), err error) {
	return EnterCbreakFD(int(os.Stdin.Fd()))
}

func EnterCbreakFD(fd int) (restore func(), err error) {
	if fd < 0 {
		return func() {}, nil
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getMode := kernel32.NewProc("GetConsoleMode")
	setMode := kernel32.NewProc("SetConsoleMode")
	handle := syscall.Handle(uintptr(fd))
	var mode uint32
	r, _, e := syscall.Syscall(getMode.Addr(), 2, uintptr(handle), uintptr(unsafe.Pointer(&mode)), 0)
	if r == 0 || e != 0 {
		return func() {}, nil
	}
	old := mode
	mode |= enableProcessedInput
	mode &^= enableLineInput | enableEchoInput | enableMouseInput
	if os.Getenv("WT_SESSION") != "" {
		mode |= enableExtendedFlags | enableVTInput
		mode &^= enableQuickEditMode
	}
	if r, _, e = syscall.Syscall(setMode.Addr(), 2, uintptr(handle), uintptr(mode), 0); r == 0 || e != 0 {
		return func() {}, nil
	}
	return func() {
		_, _, _ = syscall.Syscall(setMode.Addr(), 2, uintptr(handle), uintptr(old), 0)
	}, nil
}

func EnsureCookedFD(fd int) {
	if fd < 0 {
		return
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getMode := kernel32.NewProc("GetConsoleMode")
	setMode := kernel32.NewProc("SetConsoleMode")
	handle := syscall.Handle(uintptr(fd))
	var mode uint32
	r, _, e := syscall.Syscall(getMode.Addr(), 2, uintptr(handle), uintptr(unsafe.Pointer(&mode)), 0)
	if r == 0 || e != 0 {
		return
	}
	mode |= enableProcessedInput | enableLineInput | enableEchoInput
	_, _, _ = syscall.Syscall(setMode.Addr(), 2, uintptr(handle), uintptr(mode), 0)
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
