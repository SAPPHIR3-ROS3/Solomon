//go:build !windows

package listener

import (
	"time"

	"golang.org/x/sys/unix"
)

func ready(fd int, d time.Duration) bool {
	if fd < 0 {
		return false
	}
	var fds unix.FdSet
	fds.Bits[fd/64] |= 1 << (uint(fd) % 64)
	tv := unix.NsecToTimeval(d.Nanoseconds())
	n, err := unix.Select(fd+1, &fds, nil, nil, &tv)
	return err == nil && n > 0
}
