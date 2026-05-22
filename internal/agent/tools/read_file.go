package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureReadFile(path string) {}

type readArgs struct {
	Path      string `json:"path"`
	StartLine *int   `json:"startLine,omitempty"`
	EndLine   *int   `json:"endLine,omitempty"`
}

func readFileOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("readFile", "Read a text file relative to project root. Optional startLine/endLine (1-based, inclusive) read a line range.", map[string]any{
		"path": map[string]any{"type": "string", "description": "Path relative to project root"},
		"startLine": map[string]any{
			"type":        "integer",
			"description": "Optional first line to read (1-based, inclusive)",
			"minimum":     1,
		},
		"endLine": map[string]any{
			"type":        "integer",
			"description": "Optional last line to read (1-based, inclusive)",
			"minimum":     1,
		},
	}, []string{"path"})
}

func appendReadFileDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureReadFile)
	if err != nil {
		return err
	}
	b.addBlock("readFile", "Read a text file relative to project root. Optional startLine/endLine (1-based, inclusive) read a line range.", sig)
	return nil
}

func execReadFile(env *Env, raw json.RawMessage) (any, error) {
	var a readArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	p := resolveProjectPath(env.ProjRoot, a.Path)
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	content := string(b)
	lines := splitFileLines(content)
	totalLines := len(lines)
	result := map[string]any{"path": p, "total_lines": totalLines}
	if a.StartLine == nil && a.EndLine == nil {
		result["content"] = content
		return result, nil
	}
	start := 1
	end := totalLines
	if a.StartLine != nil {
		start = *a.StartLine
	}
	if a.EndLine != nil {
		end = *a.EndLine
	}
	if start < 1 {
		return nil, fmt.Errorf("readFile: startLine must be >= 1")
	}
	if end < 1 {
		return nil, fmt.Errorf("readFile: endLine must be >= 1")
	}
	if end < start {
		return nil, fmt.Errorf("readFile: endLine must be >= startLine")
	}
	if totalLines == 0 {
		result["content"] = ""
		result["start_line"] = start
		result["end_line"] = end
		return result, nil
	}
	if start > totalLines {
		return nil, fmt.Errorf("readFile: startLine %d beyond file (%d lines)", start, totalLines)
	}
	if end > totalLines {
		end = totalLines
	}
	slice := lines[start-1 : end]
	result["content"] = strings.Join(slice, "\n")
	result["start_line"] = start
	result["end_line"] = end
	return result, nil
}

func splitFileLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	if strings.HasSuffix(content, "\n") && len(lines) > 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func resolveProjectPath(root, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(root, filepath.Clean(p))
}
