package networkpolicy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

func DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return WrapDial((&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext)(ctx, network, address)
}

// WrapDial preserves the caller timeout and custom dial behavior when unrestricted.
func WrapDial(fallback func(context.Context, string, string) (net.Conn, error), timeout ...time.Duration) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		p, err := Read()
		if err != nil {
			return nil, err
		}
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if !p.AllowedHostname(host) {
			return nil, fmt.Errorf("Solomon access via this MagicDNS name is disabled")
		}
		dialer := net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
		if len(timeout) > 0 && timeout[0] > 0 {
			dialer.Timeout = timeout[0]
		}
		if len(p.Disabled) == 0 {
			return fallback(ctx, network, address)
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if ip.IP.IsLoopback() {
				_, port, _ := net.SplitHostPort(address)
				return fallback(ctx, network, net.JoinHostPort(ip.String(), port))
			}
		}
		rows, err := list(p)
		if err != nil {
			return nil, err
		}
		var failures []error
		for _, row := range rows {
			if !row.Enabled || !row.Up || row.Local {
				continue
			}
			for _, cidr := range row.Addresses {
				ip, _, err := net.ParseCIDR(cidr)
				if err != nil || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
					continue
				}
				candidate := dialer
				candidate.LocalAddr = &net.TCPAddr{IP: ip}
				if err := bindInterface(&candidate, row.Name); err != nil {
					return nil, err
				}
				for _, remote := range ips {
					if (ip.To4() != nil) != (remote.IP.To4() != nil) {
						continue
					}
					_, port, _ := net.SplitHostPort(address)
					target := net.JoinHostPort(remote.String(), port)
					conn, err := candidate.DialContext(ctx, network, target)
					if err == nil {
						return conn, nil
					}
					failures = append(failures, err)
				}
			}
		}
		if len(failures) == 0 {
			return nil, fmt.Errorf("no enabled network interface can connect to %s", address)
		}
		return nil, fmt.Errorf("no enabled network interface can connect to %s: %w", address, errors.Join(failures...))
	}
}
