package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"syscall"
)

// Keep the successful listener open so another process cannot take the port
// between probing it and starting the HTTP server.
func listenWithFallback(ctx context.Context, address string) (net.Listener, error) {
	host, rawPort, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return nil, err
	}
	var firstErr error
	for candidate := port; candidate <= 65535; candidate++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		listener, err := listenForDaemon(ctx, net.JoinHostPort(host, strconv.Itoa(candidate)))
		if err == nil {
			if firstErr != nil {
				fmt.Fprintf(os.Stderr, "Solomon: port %d unavailable (%v); using port %d\n", port, firstErr, candidate)
			}
			return listener, nil
		}
		if firstErr == nil {
			firstErr = err
		}
		windowsPortError := runtime.GOOS == "windows" && (errors.Is(err, syscall.Errno(10048)) || errors.Is(err, syscall.Errno(10013)))
		if !errors.Is(err, syscall.EADDRINUSE) && !errors.Is(err, syscall.EACCES) && !windowsPortError {
			return nil, fmt.Errorf("start Solomon on %s: %w", address, err)
		}
	}
	return nil, fmt.Errorf("cannot start Solomon: no available TCP port from %d to 65535 (first failure: %w)", port, firstErr)
}
