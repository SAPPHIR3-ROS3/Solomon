package networkpolicy

import (
	"golang.org/x/sys/unix"
	"net"
	"syscall"
)

func bindInterface(d *net.Dialer, name string) error {
	d.Control = func(network, address string, raw syscall.RawConn) error {
		var bindErr error
		err := raw.Control(func(fd uintptr) {
			bindErr = unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, name)
		})
		if err != nil {
			return err
		}
		return bindErr
	}
	return nil
}
