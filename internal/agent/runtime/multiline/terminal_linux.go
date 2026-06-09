//go:build linux || aix || solaris || zos

package multiline

import (
	"os"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func EnsureCookedTTY() {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return
	}
	t, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return
	}
	t.Iflag |= unix.ICRNL | unix.IXON | unix.BRKINT
	t.Oflag |= unix.OPOST | unix.ONLCR
	t.Lflag |= unix.ECHO | unix.ICANON | unix.ISIG | unix.IEXTEN
	t.Cflag |= unix.CS8
	t.Cc[unix.VMIN] = 1
	t.Cc[unix.VTIME] = 0
	_ = unix.IoctlSetTermios(fd, unix.TCSETS, t)
}

func EnterRawStdin() (restore func(), err error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return func() {}, nil
	}
	cur, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return func() {}, err
	}
	saved := *cur
	next := *cur
	next.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	next.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	next.Cflag &^= unix.CSIZE | unix.PARENB
	next.Cflag |= unix.CS8
	next.Cc[unix.VMIN] = 1
	next.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &next); err != nil {
		return func() {}, err
	}
	return func() { _ = unix.IoctlSetTermios(fd, unix.TCSETS, &saved) }, nil
}

func FlushStdin() {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return
	}
	cur, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return
	}
	saved := *cur
	flush := saved
	flush.Lflag &^= unix.ICANON | unix.ISIG
	flush.Cc[unix.VMIN] = 0
	flush.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &flush); err != nil {
		return
	}
	buf := make([]byte, 256)
	for {
		n, err := unix.Read(fd, buf)
		if n == 0 || err != nil {
			break
		}
	}
	_ = unix.IoctlSetTermios(fd, unix.TCSETS, &saved)
}
