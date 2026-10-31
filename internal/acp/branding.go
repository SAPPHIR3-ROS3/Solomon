package acp

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logo"

func (s *Server) agentInfo() map[string]any {
	info := map[string]any{"name": "solomon", "title": "Solomon", "version": s.version}
	if branding, err := logo.Branding(); err == nil {
		info["_meta"] = map[string]any{"solomon/branding": branding}
	}
	return info
}
