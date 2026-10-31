package server

import (
	"context"
	"net"
	"strconv"
	"testing"
)

func TestListenWithFallbackSkipsOccupiedPort(t *testing.T) {
	busy, err := listenForDaemon(context.Background(), "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	port := busy.Addr().(*net.TCPAddr).Port
	listener, err := listenWithFallback(context.Background(), net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if got := listener.Addr().(*net.TCPAddr).Port; got <= port {
		t.Fatalf("port = %d, want > %d", got, port)
	}
}

func TestListenWithFallbackHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := listenWithFallback(ctx, "127.0.0.1:64000"); err != context.Canceled {
		t.Fatalf("error = %v", err)
	}
}
