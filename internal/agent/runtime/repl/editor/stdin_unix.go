//go:build !windows

package editor

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func stdinReady(d time.Duration) bool {
	var fds unix.FdSet
	fd := int(os.Stdin.Fd())
	fds.Bits[fd/64] |= 1 << (fd % 64)
	tv := unix.NsecToTimeval(d.Nanoseconds())
	n, err := unix.Select(fd+1, &fds, nil, nil, &tv)
	return err == nil && n > 0
}
