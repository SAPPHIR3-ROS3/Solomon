package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/cloak"
)

type cloakBrowserArgs struct {
	Tool   string         `json:"tool"`
	Args   map[string]any `json:"args"`
	Intent string         `json:"intent"`
}

func execCloakBrowser(ctx context.Context, env *Env, raw json.RawMessage) (any, error) {
	var a cloakBrowserArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	if strings.TrimSpace(a.Tool) == "" {
		return nil, fmt.Errorf("cloakBrowser: tool is required")
	}
	if env == nil || env.Cloak == nil {
		return nil, fmt.Errorf("cloakBrowser: native Cloak client unavailable")
	}
	if a.Args == nil {
		a.Args = map[string]any{}
	}
	if a.Tool == "upload_file" {
		if err := cloakUploadPaths(env.ProjRoot, a.Args); err != nil {
			return nil, err
		}
	}
	return env.Cloak.CallTool(ctx, a.Tool, a.Args, a.Intent)
}

func cloakDeferredTool() deferredTool {
	terms := "cloak browser automation tabs navigate snapshot click fill playwright network performance memory heap console css webauthn resize upload"
	for _, tool := range cloak.Tools() {
		terms += " " + tool.Name + " " + tool.Description
	}
	return deferredTool{
		Name:        "cloakBrowser",
		Description: "Control the native CloakBrowser through its supported browser tools. Pass the supported tool name and its argument object; intent is required separately. No MCP installation is needed.",
		SDKCall:     "CloakBrowser(tool string, args map[string]any, intent string) (map[string]any, error)",
		Mode:        "deferred",
		SearchTerms: terms,
	}
}

func addCloakCatalog(entry map[string]string) {
	tools := make([]map[string]any, 0)
	names := make([]string, 0)
	for _, tool := range cloak.Tools() {
		names = append(names, tool.Name)
		tools = append(tools, map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  tool.Parameters,
		})
	}
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tool":   map[string]any{"type": "string", "enum": names},
			"args":   map[string]any{"type": "object", "description": "Arguments matching the selected supported tool schema"},
			"intent": map[string]any{"type": "string", "minLength": 1, "pattern": "\\S", "description": "Brief non-empty purpose for this browser action"},
		},
		"required": []string{"tool", "args", "intent"},
	}
	if raw, err := json.Marshal(schema); err == nil {
		entry["parameters"] = string(raw)
	}
	if raw, err := json.Marshal(tools); err == nil {
		entry["supported_tools"] = string(raw)
	}
}

func cloakUploadPaths(root string, args map[string]any) error {
	if strings.TrimSpace(root) == "" {
		return fmt.Errorf("cloakBrowser: upload requires a workspace")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return err
	}
	paths, ok := args["filePaths"].([]any)
	if !ok || len(paths) == 0 || len(paths) > 20 {
		return fmt.Errorf("cloakBrowser: filePaths must contain 1 to 20 files")
	}
	resolved := make([]string, 0, len(paths))
	var total int64
	for _, raw := range paths {
		path, ok := raw.(string)
		if !ok || path == "" {
			return fmt.Errorf("cloakBrowser: invalid upload path")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(canonical, path)
		}
		path, err = filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(canonical, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("cloakBrowser: upload must remain inside the workspace")
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		total += info.Size()
		if !info.Mode().IsRegular() || total > 50*1024*1024 {
			return fmt.Errorf("cloakBrowser: upload requires regular files totaling at most 50 MiB")
		}
		resolved = append(resolved, path)
	}
	args["filePaths"] = resolved
	return nil
}
