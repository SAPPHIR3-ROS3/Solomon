package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureEditFile(path, oldString, newString, intent string) {}

func signatureEditFileDelete(path string, delete bool, intent string) {}

type editArgs struct {
	Path      string `json:"path"`
	OldString string `json:"oldString"`
	NewString string `json:"newString"`
	Intent    string `json:"intent"`
	Delete    bool   `json:"delete,omitempty"`
}

func editFileOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("editFile", "Replace oldString once with newString; use empty oldString to create or overwrite; set delete=true to remove the file at path.", map[string]any{
		"path":      map[string]any{"type": "string", "description": "Path relative to project root"},
		"oldString": map[string]any{"type": "string", "description": "Substring to replace once; empty means create/overwrite when delete is false"},
		"newString": map[string]any{"type": "string", "description": "New content or replacement text; ignored when delete is true"},
		"delete":    map[string]any{"type": "boolean", "description": "When true, deletes the file at path instead of editing it"},
		"intent":    map[string]any{"type": "string", "description": "Brief phrase describing the purpose of this edit or deletion"},
	}, []string{"path", "intent"})
}

func appendEditFileDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureEditFile)
	if err != nil {
		return err
	}
	delSig, err := tooling.FuncSignature(signatureEditFileDelete)
	if err != nil {
		return err
	}
	b.addBlock(
		"editFile",
		"Replace oldString once with newString; use empty oldString to create or overwrite; use delete=true with path and intent to remove a file ("+delSig+"). Requires intent (brief purpose).",
		sig,
	)
	return nil
}

func execEditFile(env *Env, raw json.RawMessage) (any, error) {
	var a editArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	if strings.TrimSpace(a.Intent) == "" {
		return nil, fmt.Errorf("intent must be a non-empty brief phrase")
	}
	p := resolveProjectPath(env.ProjRoot, a.Path)
	if env.ActivateInstructionsFromAbsPath != nil {
		env.ActivateInstructionsFromAbsPath(p)
	}
	if a.Delete {
		if err := os.Remove(p); err != nil {
			if os.IsNotExist(err) {
				return map[string]any{"ok": false, "reason": "file not found"}, nil
			}
			return nil, err
		}
		env.CheckpointStageProjAbs(p)
		return map[string]any{"ok": true, "action": "deleted", "intent": a.Intent}, nil
	}
	if a.OldString == "" && a.NewString == "" {
		return nil, fmt.Errorf("editFile refuses empty overwrite")
	}
	if a.OldString == "" {
		if err := os.WriteFile(p, []byte(a.NewString), 0o600); err != nil {
			return nil, err
		}
		env.CheckpointStageProjAbs(p)
		return map[string]any{"ok": true, "action": "created_or_overwrite", "intent": a.Intent}, nil
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
	return map[string]any{"ok": true, "action": "edited", "intent": a.Intent}, nil
}
