package tools

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureTree(path string) {}

type treeArgs struct {
	Path              string `json:"path"`
	MaxDepth          *int   `json:"maxDepth"`
	MaxEntries        *int   `json:"maxEntries"`
	IncludeHidden     bool   `json:"includeHidden"`
	RespectGitignore  *bool  `json:"respectGitignore"`
}

func treeOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("tree", "Render an ASCII directory tree under a project-relative path.", map[string]any{
		"path": map[string]any{
			"type":        "string",
			"description": "Directory relative to project root (default .)",
		},
		"maxDepth": map[string]any{
			"type":        "integer",
			"description": "Maximum directory depth to expand (default 6)",
			"minimum":     1,
		},
		"maxEntries": map[string]any{
			"type":        "integer",
			"description": "Maximum nodes to include (default 800)",
			"minimum":     1,
		},
		"includeHidden": map[string]any{
			"type":        "boolean",
			"description": "Include dotfiles and dot-directories (default false)",
		},
		"respectGitignore": map[string]any{
			"type":        "boolean",
			"description": "Honor .gitignore rules (default true)",
		},
	}, []string{})
}

func appendTreeDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureTree)
	if err != nil {
		return err
	}
	b.addBlock("tree", "ASCII directory tree with depth and entry limits. Dirs listed before files at each level.", sig)
	return nil
}

func execTree(env *Env, raw json.RawMessage) (any, error) {
	var a treeArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	abs := resolveProjectPath(env.ProjRoot, a.Path)
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("tree: not a directory: %s", a.Path)
	}
	maxDepth := defaultTreeMaxDepth
	if a.MaxDepth != nil && *a.MaxDepth > 0 {
		maxDepth = *a.MaxDepth
	}
	maxEntries := defaultTreeMaxEntries
	if a.MaxEntries != nil && *a.MaxEntries > 0 {
		maxEntries = *a.MaxEntries
	}
	respect := true
	if a.RespectGitignore != nil {
		respect = *a.RespectGitignore
	}
	text, count, truncated, err := buildDirectoryTree(env.ProjRoot, abs, maxDepth, maxEntries, dirBrowseOpts{
		IncludeHidden:    a.IncludeHidden,
		RespectGitignore: respect,
	})
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"path":    relPathOrDot(env.ProjRoot, abs),
		"tree":    text,
		"entries": count,
	}
	if truncated {
		out["truncated"] = true
	}
	return out, nil
}
