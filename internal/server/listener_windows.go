package server

import (
	"context"
	"errors"
	"net"
	"syscall"
)

func listenForDaemon(ctx context.Context, address string) (net.Listener, error) {
	config := net.ListenConfig{Control: func(_, _ string, connection syscall.RawConn) error {
		var socketErr error
		err := connection.Control(func(handle uintptr) {
			// Winsock defines SO_EXCLUSIVEADDRUSE as the complement of
			// SO_REUSEADDR. Prevent wildcard/specific listeners sharing a port.
			socketErr = syscall.SetsockoptInt(syscall.Handle(handle), syscall.SOL_SOCKET, ^syscall.SO_REUSEADDR, 1)
		})
		return errors.Join(err, socketErr)
	}}
	return config.Listen(ctx, "tcp4", address)
}
