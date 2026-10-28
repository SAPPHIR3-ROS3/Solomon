//go:build !linux

package networkpolicy

import "net"

// The source address constrains the selected interface on these platforms.
func bindInterface(d *net.Dialer, name string) error { return nil }
