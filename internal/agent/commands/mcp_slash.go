package commands

import (
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
		fmt.Fprintln(d.Out, "(no MCP servers in config)")
		return nil
	}
	for _, s := range p.Servers {
		col3 := s.Command
		if strings.TrimSpace(s.URL) != "" {
			col3 = redactMCPURL(s.URL)
		}
		fmt.Fprintf(d.Out, "%s\t%s\t%s\n", s.Name, s.Type, col3)
	}
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
