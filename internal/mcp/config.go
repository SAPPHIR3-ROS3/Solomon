package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

const (
	TransportStdio          = "stdio"
	TransportStreamableHTTP = "streamable-http"
)

type Config struct {
	Servers []ServerConfig
	Path    string
}

type ServerConfig struct {
	Name      string            `json:"-"`
	Type      string            `json:"type"`
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	CWD       string            `json:"cwd"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	Timeout   int               `json:"timeout"`
	Allow     []string          `json:"allow"`
	Deny      []string          `json:"deny"`
	Fallback  bool              `json:"-"`
	SortIndex int               `json:"-"`
}

func LoadConfig() (*Config, error) {
	p, err := paths.MCPConfigPath()
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "MCP config path resolve failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{Path: p}, nil
		}
		err = fmt.Errorf("read mcp config %q: %w", p, err)
		logging.Log(logging.ERROR_LOG_LEVEL, "MCP config read failed", logging.LogOptions{Params: map[string]any{"path": p, "err": err.Error()}})
		return nil, err
	}
	cfg, err := ParseConfig(b)
	if err != nil {
		err = fmt.Errorf("parse mcp config %q: %w", p, err)
		logging.Log(logging.ERROR_LOG_LEVEL, "MCP config parse failed", logging.LogOptions{Params: map[string]any{"path": p, "err": err.Error()}})
		return nil, err
	}
	cfg.Path = p
	return cfg, nil
}

func ConfiguredServerCount() (int, error) {
	p, err := paths.MCPConfigPath()
	if err != nil {
		return 0, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("read mcp config %q: %w", p, err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		return 0, fmt.Errorf("malformed JSON: %w", err)
	}
	rawServers, ok := doc["mcpServers"]
	if !ok {
		return 0, nil
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(rawServers, &servers); err != nil {
		return 0, fmt.Errorf("top-level mcpServers must be an object: %w", err)
	}
	return len(servers), nil
}

func ParseConfig(raw []byte) (*Config, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("malformed JSON: %w", err)
	}
	rawServers, ok := doc["mcpServers"]
	if !ok {
		return nil, fmt.Errorf("top-level mcpServers is required")
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(rawServers, &servers); err != nil {
		return nil, fmt.Errorf("top-level mcpServers must be an object: %w", err)
	}
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	cfg := &Config{}
	for i, name := range names {
		sc, err := parseServer(name, i+1, servers[name])
		if err != nil {
			return nil, fmt.Errorf("server %q: %w", name, err)
		}
		cfg.Servers = append(cfg.Servers, sc)
	}
	return cfg, nil
}

func parseServer(name string, index int, raw json.RawMessage) (ServerConfig, error) {
	var expanded any
	if err := json.Unmarshal(raw, &expanded); err != nil {
		return ServerConfig{}, fmt.Errorf("invalid server object: %w", err)
	}
	expanded, err := expandEnvValue(expanded)
	if err != nil {
		return ServerConfig{}, err
	}
	b, err := json.Marshal(expanded)
	if err != nil {
		return ServerConfig{}, err
	}
	var sc ServerConfig
	if err := json.Unmarshal(b, &sc); err != nil {
		return ServerConfig{}, fmt.Errorf("invalid fields: %w", err)
	}
	sc.Name = strings.TrimSpace(name)
	sc.SortIndex = index
	if sc.Name == "" {
		sc.Fallback = true
		sc.Name = fmt.Sprintf("server%d", index)
	}
	if sc.Env == nil {
		sc.Env = map[string]string{}
	}
	if sc.Headers == nil {
		sc.Headers = map[string]string{}
	}
	if sc.Allow == nil {
		sc.Allow = []string{}
	}
	if sc.Deny == nil {
		sc.Deny = []string{}
	}
	return validateServer(sc)
}

func validateServer(sc ServerConfig) (ServerConfig, error) {
	sc.Type = strings.TrimSpace(sc.Type)
	if sc.Type == "" {
		if strings.TrimSpace(sc.URL) != "" {
			sc.Type = TransportStreamableHTTP
		} else {
			sc.Type = TransportStdio
		}
	}
	switch sc.Type {
	case TransportStdio:
		if strings.TrimSpace(sc.Command) == "" {
			return ServerConfig{}, fmt.Errorf("command is required for stdio transport")
		}
	case TransportStreamableHTTP:
		if strings.TrimSpace(sc.URL) == "" {
			return ServerConfig{}, fmt.Errorf("url is required for streamable-http transport")
		}
		u, err := url.Parse(sc.URL)
		if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return ServerConfig{}, fmt.Errorf("url must be a valid http or https URL")
		}
	default:
		return ServerConfig{}, fmt.Errorf("unknown transport type %q", sc.Type)
	}
	return sc, nil
}

func (sc ServerConfig) ToolAllowed(name string) bool {
	allowed := len(sc.Allow) == 0
	for _, item := range sc.Allow {
		if item == name {
			allowed = true
			break
		}
	}
	for _, item := range sc.Deny {
		if item == name {
			return false
		}
	}
	return allowed
}

var envToken = regexp.MustCompile(`\$([A-Za-z0-9_]+)`)

func expandEnvValue(v any) (any, error) {
	switch x := v.(type) {
	case string:
		return expandEnvString(x)
	case []any:
		for i := range x {
			y, err := expandEnvValue(x[i])
			if err != nil {
				return nil, err
			}
			x[i] = y
		}
		return x, nil
	case map[string]any:
		for k, v := range x {
			y, err := expandEnvValue(v)
			if err != nil {
				return nil, err
			}
			x[k] = y
		}
		return x, nil
	default:
		return v, nil
	}
}

func expandEnvString(s string) (string, error) {
	var missing string
	out := envToken.ReplaceAllStringFunc(s, func(tok string) string {
		name := tok[1:]
		v, ok := os.LookupEnv(name)
		if !ok {
			missing = name
			return tok
		}
		return v
	})
	if missing != "" {
		return "", fmt.Errorf("missing system environment variable %q", missing)
	}
	return out, nil
}
