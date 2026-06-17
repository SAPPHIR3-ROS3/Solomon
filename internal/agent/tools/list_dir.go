package tools

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureListDir(path string) {}

type listDirArgs struct {
	Path              string `json:"path"`
	IncludeHidden     bool   `json:"includeHidden"`
	RespectGitignore  *bool  `json:"respectGitignore"`
}

func listDirOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("listDir", "List files and immediate subdirectories in a project-relative directory (non-recursive).", map[string]any{
		"path": map[string]any{
			"type":        "string",
			"description": "Directory relative to project root (default .)",
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

func appendListDirDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureListDir)
	if err != nil {
		return err
	}
	b.addBlock("listDir", "List one directory level (dirs first, then files). Optional includeHidden and respectGitignore.", sig)
	return nil
}

func execListDir(env *Env, raw json.RawMessage) (any, error) {
	var a listDirArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	abs := resolveProjectPath(env.ProjRoot, a.Path)
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("listDir: not a directory: %s", a.Path)
	}
	respect := true
	if a.RespectGitignore != nil {
		respect = *a.RespectGitignore
	}
	entries, err := listDirectoryEntries(env.ProjRoot, abs, dirBrowseOpts{
		IncludeHidden:    a.IncludeHidden,
		RespectGitignore: respect,
	})
	if err != nil {
		return nil, err
	}
	list := make([]map[string]any, len(entries))
	for i, e := range entries {
		row := map[string]any{"name": e.Name, "type": e.Type}
		if e.Size > 0 {
			row["size"] = e.Size
		}
		list[i] = row
	}
	return map[string]any{
		"path":    relPathOrDot(env.ProjRoot, abs),
		"entries": list,
		"count":   len(list),
	}, nil
}
