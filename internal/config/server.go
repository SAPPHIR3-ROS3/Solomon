package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

const DefaultServerBind = "127.0.0.1"
const DefaultServerPort = 8443

type Server struct {
	Bind      string `toml:"bind,omitempty"`
	Port      int    `toml:"port,omitempty"`
	StaticDir string `toml:"static_dir,omitempty"`
	TLSCert   string `toml:"tls_cert,omitempty"`
	TLSKey    string `toml:"tls_key,omitempty"`
}

func (r *Root) EffectiveServerAddr() string {
	host := DefaultServerBind
	port := DefaultServerPort
	if r != nil {
		if b := strings.TrimSpace(r.Server.Bind); b != "" {
			if h, p, err := net.SplitHostPort(b); err == nil {
				host = h
				if n, err := strconv.Atoi(p); err == nil && n > 0 {
					port = n
				}
			} else {
				host = b
			}
		}
		if r.Server.Port > 0 {
			port = r.Server.Port
		}
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func (r *Root) EffectiveServerStaticDir() string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(r.Server.StaticDir)
}

func (r *Root) EffectiveServerTLSPaths() (cert, key string) {
	if r == nil {
		return "", ""
	}
	return strings.TrimSpace(r.Server.TLSCert), strings.TrimSpace(r.Server.TLSKey)
}

func ParseServeAddrOverride(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if !strings.Contains(raw, ":") {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return "", fmt.Errorf("invalid port %q", raw)
		}
		return net.JoinHostPort(DefaultServerBind, strconv.Itoa(n)), nil
	}
	host, port, err := net.SplitHostPort(raw)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(host) == "" {
		host = DefaultServerBind
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("invalid port in %q", raw)
	}
	return net.JoinHostPort(host, port), nil
}
