package commands

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/internal/mcp"
)

func SlashMCP(d Deps) error {
	p, err := solomonmcp.LoadConfig()
	if err != nil {
		return err
	}
	if len(p.Servers) == 0 {
		PrintSystem(d.Out, "(no MCP servers in config)")
		return nil
	}
	var buf bytes.Buffer
	for _, s := range p.Servers {
		col3 := s.Command
		if strings.TrimSpace(s.URL) != "" {
			col3 = redactMCPURL(s.URL)
		}
		fmt.Fprintf(&buf, "%s\t%s\t%s\n", s.Name, s.Type, col3)
	}
	PrintSystem(d.Out, buf.String())
	return nil
}

func redactMCPURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.User = nil
	return u.String()
}
