package main

import (
	"net"
	"net/http"
	"time"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

// ServerBridge exposes the loopback API gateway to the native WebView.
// The gateway forwards requests and streams to the daemon discovered in state.
type ServerBridge struct{ url string }

func (b ServerBridge) URL() string { return b.url }

func startDesktopProxy() (*ServerBridge, func(), error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	server := &http.Server{Handler: serverruntime.DesktopProxy(), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(listener) }()
	return &ServerBridge{url: "http://" + listener.Addr().String()}, func() { _ = server.Close() }, nil
}
