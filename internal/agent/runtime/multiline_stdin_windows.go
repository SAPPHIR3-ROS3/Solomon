//go:build windows

package agentruntime

import (
	"os"
	"syscall"
	"unsafe"

	readline "github.com/chzyer/readline"
)

const enableMouseInput = 0x0010

func platformStdin() stdinReadCloser {
	return readline.NewRawReader()
}

func prepareConsoleInput() func() {
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
	_, _, _ = syscall.Syscall(setMode.Addr(), 2, uintptr(handle), uintptr(mode), 0)
	return func() {
		_, _, _ = syscall.Syscall(setMode.Addr(), 2, uintptr(handle), uintptr(old), 0)
	}
}
