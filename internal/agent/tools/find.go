package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureFind(pattern string, files bool) {}

func findOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("find", "Search the project: files=true lists paths matching a glob pattern; files=false searches file contents with a Go regexp.", map[string]any{
		"pattern": map[string]any{"type": "string", "description": "Glob pattern when files=true; Go regexp when files=false"},
		"files":   map[string]any{"type": "boolean", "description": "true=list matching paths by mtime; false=search text in files"},
		"path": map[string]any{
			"type":        "string",
			"description": "Directory relative to project root (default .)",
		},
		"pathGlob": map[string]any{
			"type":        "string",
			"description": "When files=false, optional extra glob filter on relative paths",
		},
		"outputMode": map[string]any{
			"type":        "string",
			"description": "When files=false: content (default), files_with_matches, or count",
		},
		"caseInsensitive": map[string]any{"type": "boolean"},
		"contextBefore":   map[string]any{"type": "integer", "minimum": 0},
		"contextAfter":    map[string]any{"type": "integer", "minimum": 0},
		"context":         map[string]any{"type": "integer", "minimum": 0},
		"headLimit":       map[string]any{"type": "integer", "minimum": 1},
		"multiline":       map[string]any{"type": "boolean"},
		"timeoutSeconds": map[string]any{
			"type":        "integer",
			"description": "Optional timeout in seconds; omit to use default 60",
		},
	}, []string{"pattern", "files"})
}

func appendFindDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureFind)
	if err != nil {
		return err
	}
	b.addBlock("find", "files=true: glob listing sorted by mtime desc. files=false: regexp content search. Both modes support optional timeoutSeconds (default 60s), pathGlob, outputMode, context lines, headLimit.", sig)
	return nil
}

type findArgs struct {
	Pattern          string `json:"pattern"`
	Files            bool   `json:"files"`
	Path             string `json:"path"`
	PathGlob         string `json:"pathGlob"`
	OutputMode       string `json:"outputMode"`
	CaseInsensitive  bool   `json:"caseInsensitive"`
	ContextBefore    *int   `json:"contextBefore"`
	ContextAfter     *int   `json:"contextAfter"`
	Context          *int   `json:"context"`
	HeadLimit        *int   `json:"headLimit"`
	Multiline        bool   `json:"multiline"`
	TimeoutSeconds   *int   `json:"timeoutSeconds,omitempty"`
}

func execFind(ctx context.Context, env *Env, raw json.RawMessage) (any, error) {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		return nil, err
	}
	var a findArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	if sec := parseOptionalTimeoutSecs(rawMap); sec != nil && *sec > 0 {
		a.TimeoutSeconds = sec
	} else {
		a.TimeoutSeconds = nil
	}
	if a.Pattern == "" {
		return nil, fmt.Errorf("pattern required")
	}
	root := resolveProjectPath(env.ProjRoot, a.Path)
	if a.Files {
		return execFindPaths(ctx, env, root, &a)
	}
	return execFindText(ctx, env, root, &a)
}

const findDefaultTimeout = time.Minute

func findTimeoutDuration(a *findArgs) time.Duration {
	if a == nil || a.TimeoutSeconds == nil || *a.TimeoutSeconds <= 0 {
		return findDefaultTimeout
	}
	return time.Duration(*a.TimeoutSeconds) * time.Second
}

func findRunContext(ctx context.Context, a *findArgs) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, findTimeoutDuration(a))
}
