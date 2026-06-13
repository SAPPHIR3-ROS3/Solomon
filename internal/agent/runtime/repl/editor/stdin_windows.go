//go:build windows

package editor

import (
	"os"
	"syscall"
	"time"
	"unsafe"
)

func stdinReady(d time.Duration) bool {
	deadline := time.Now().Add(d)
	for {
		if stdinHasInput() {
			return true
		}
		if time.Until(deadline) <= 0 {
			return false
		}
		time.Sleep(time.Millisecond)
	}
}

func stdinHasInput() bool {
	handle := syscall.Handle(os.Stdin.Fd())
	kernel32 := syscall.NewLazyDLL("kernel32.dll")

	var mode uint32
	getMode := kernel32.NewProc("GetConsoleMode")
	if r, _, _ := syscall.Syscall(getMode.Addr(), 2, uintptr(handle), uintptr(unsafe.Pointer(&mode)), 0); r != 0 {
		var events uint32
		getEvents := kernel32.NewProc("GetNumberOfConsoleInputEvents")
		r, _, _ := syscall.Syscall(getEvents.Addr(), 2, uintptr(handle), uintptr(unsafe.Pointer(&events)), 0)
		return r != 0 && events > 0
	}

	var avail uint32
	peek := kernel32.NewProc("PeekNamedPipe")
	r, _, _ := syscall.Syscall6(peek.Addr(), 6, uintptr(handle), 0, 0, 0, uintptr(unsafe.Pointer(&avail)), 0)
	return r != 0 && avail > 0
}
