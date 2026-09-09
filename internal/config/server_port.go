package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ParseServerPort validates the numeric port representation used by the
// environment and config file.
func ParseServerPort(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("server port is empty")
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("server port must be a TCP port between 1 and 65535")
	}
	return port, nil
}

func ValidateServerPort(port int) error {
	if port == 0 {
		return nil
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("server_port must be a TCP port between 1 and 65535")
	}
	return nil
}

func (r *Root) EffectiveServerPort() int {
	if r == nil || r.ServerPort == 0 {
		return DefaultServerPort
	}
	return r.ServerPort
}

// ResolveServerPort gives an explicit process environment value precedence,
// persists it to config.toml, and otherwise uses the persisted config value.
// A missing value is deterministic rather than an instruction to bind an
// ephemeral port.
func ResolveServerPort() (int, error) {
	cfg, err := LoadOptional()
	if err != nil {
		return 0, err
	}
	if err := ValidateServerPort(cfg.ServerPort); err != nil {
		return 0, err
	}

	port := cfg.EffectiveServerPort()
	if raw, ok := os.LookupEnv(ServerPortEnv); ok && strings.TrimSpace(raw) != "" {
		port, err = ParseServerPort(raw)
		if err != nil {
			return 0, fmt.Errorf("%s: %w", ServerPortEnv, err)
		}
	}

	if cfg.ServerPort != port {
		cfg.ServerPort = port
		if err := Save(cfg); err != nil {
			return 0, fmt.Errorf("persist server port: %w", err)
		}
	}
	return port, nil
}
