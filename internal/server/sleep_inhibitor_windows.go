package server

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
)

func acquireSleepInhibitor() (func(), error) {
	stop, done := make(chan struct{}), make(chan struct{})
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(done)
		proc := syscall.NewLazyDLL("kernel32.dll").NewProc("SetThreadExecutionState")
		result, _, err := proc.Call(0x80000001) // ES_CONTINUOUS | ES_SYSTEM_REQUIRED
		if result == 0 {
			ready <- fmt.Errorf("prevent sleep: %w", err)
			return
		}
		ready <- nil
		<-stop
		proc.Call(0x80000000) // Release on the same OS thread.
	}()
	if err := <-ready; err != nil {
		return nil, err
	}
	var once sync.Once
	return func() { once.Do(func() { close(stop); <-done }) }, nil
}
