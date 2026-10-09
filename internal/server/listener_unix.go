//go:build !windows

package server

import (
	"context"
	"net"
)

func listenForDaemon(ctx context.Context, address string) (net.Listener, error) {
	return (&net.ListenConfig{}).Listen(ctx, "tcp4", address)
}
