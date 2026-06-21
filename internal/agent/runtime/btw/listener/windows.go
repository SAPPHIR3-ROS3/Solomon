//go:build windows

package listener

import (
	"syscall"
	"time"
	"unsafe"
)

const (
	waitObject0 = 0
)

func ready(fd int, d time.Duration) bool {
	if fd < 0 {
		return false
	}
	handle := syscall.Handle(uintptr(fd))
	kernel32 := syscall.NewLazyDLL("kernel32.dll")

	var mode uint32
	getMode := kernel32.NewProc("GetConsoleMode")
	if r, _, _ := syscall.Syscall(getMode.Addr(), 2, uintptr(handle), uintptr(unsafe.Pointer(&mode)), 0); r != 0 {
		wait := kernel32.NewProc("WaitForSingleObject")
		r, _, _ = syscall.Syscall(wait.Addr(), 2, uintptr(handle), uintptr(durationMillis(d)), 0)
		if r != waitObject0 {
			return false
		}
		var events uint32
		getEvents := kernel32.NewProc("GetNumberOfConsoleInputEvents")
		r, _, _ = syscall.Syscall(getEvents.Addr(), 2, uintptr(handle), uintptr(unsafe.Pointer(&events)), 0)
		return r != 0 && events > 0
	}

	deadline := time.Now().Add(d)
	for {
		if pipeHasInput(handle, kernel32) {
			return true
		}
		if time.Until(deadline) <= 0 {
			return false
		}
		time.Sleep(time.Millisecond)
	}
}

func durationMillis(d time.Duration) uint32 {
	if d <= 0 {
		return 0
	}
	ms := d.Milliseconds()
	if ms <= 0 {
		return 1
	}
	if ms > 1<<32-1 {
		return 1<<32 - 1
	}
	return uint32(ms)
}

func pipeHasInput(handle syscall.Handle, kernel32 *syscall.LazyDLL) bool {
	var avail uint32
	peek := kernel32.NewProc("PeekNamedPipe")
	r, _, _ := syscall.Syscall6(peek.Addr(), 6, uintptr(handle), 0, 0, 0, uintptr(unsafe.Pointer(&avail)), 0)
	return r != 0 && avail > 0
}
