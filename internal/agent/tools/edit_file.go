package tools

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureEditFile(path, oldString, newString string) {}

type editArgs struct {
	Path      string `json:"path"`
	OldString string `json:"oldString"`
	NewString string `json:"newString"`
}

func editFileOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("editFile", "Replace oldString once with newString, or write newString when oldString is empty.", map[string]any{
		"path":      map[string]any{"type": "string", "description": "Path relative to project root"},
		"oldString": map[string]any{"type": "string", "description": "Substring to replace once; empty means create/overwrite per tool semantics"},
		"newString": map[string]any{"type": "string", "description": "New content or replacement text"},
	}, []string{"path", "oldString", "newString"})
}

func appendEditFileDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureEditFile)
	if err != nil {
		return err
	}
	b.addBlock("editFile", "Replace oldString once with newString, or write newString when oldString empty.", sig)
	return nil
}

func execEditFile(env *Env, raw json.RawMessage) (any, error) {
	var a editArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	p := resolveProjectPath(env.ProjRoot, a.Path)
	if a.OldString == "" {
		if err := os.WriteFile(p, []byte(a.NewString), 0o600); err != nil {
			return nil, err
		}
		env.CheckpointStageProjAbs(p)
		return map[string]any{"ok": true, "action": "created_or_overwrite"}, nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	s := string(b)
	if !strings.Contains(s, a.OldString) {
		return map[string]any{"ok": false, "reason": "oldString not found"}, nil
	}
	s = strings.Replace(s, a.OldString, a.NewString, 1)
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		return nil, err
	}
	env.CheckpointStageProjAbs(p)
	return map[string]any{"ok": true, "action": "edited"}, nil
}
