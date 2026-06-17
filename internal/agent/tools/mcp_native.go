package tools

import (
	"encoding/json"
	"strings"

	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
)

func mcpToolAllowed(env *Env, name string) bool {
	return env != nil && env.MCP != nil && env.MCP.HasTool(name)
}

func appendMCPSearchHits(env *Env, q string, result map[string]any) {
	if env == nil || env.MCP == nil {
		return
	}
	list, _ := result["tools"].([]map[string]string)
	if list == nil {
		list = []map[string]string{}
	}
	seen := map[string]struct{}{}
	for _, e := range list {
		seen[e["name"]] = struct{}{}
	}
	for _, t := range env.MCP.Catalog() {
		if !mcpCatalogEntryMatches(q, t) {
			continue
		}
		if _, ok := seen[t.Name]; ok {
			continue
		}
		entry := map[string]string{
			"name":        t.Name,
			"description": t.Description,
			"origin_mode": "native",
		}
		if len(t.Schema) > 0 {
			if b, err := json.Marshal(t.Schema); err == nil {
				entry["parameters"] = string(b)
			}
		}
		list = append(list, entry)
		seen[t.Name] = struct{}{}
	}
	result["tools"] = list
	result["count"] = len(list)
}

func mcpCatalogEntryMatches(q string, t solomonmcp.CatalogEntry) bool {
	if mcpQueryBroad(q) {
		return true
	}
	hay := strings.ToLower(t.Name + " " + t.Server + " " + t.Tool + " " + t.Description)
	if strings.Contains(hay, q) {
		return true
	}
	words := significantQueryWords(q)
	if len(words) == 0 {
		return false
	}
	for _, w := range words {
		if wordMatchesHay(hay, w) {
			return true
		}
	}
	return false
}

func mcpQueryBroad(q string) bool {
	q = strings.TrimSpace(strings.ToLower(q))
	if q == "" {
		return false
	}
	if strings.Contains(q, "mcp") {
		return true
	}
	for _, w := range []string{"modelcontextprotocol", "remote", "integration"} {
		if strings.Contains(q, w) {
			return true
		}
	}
	return false
}
