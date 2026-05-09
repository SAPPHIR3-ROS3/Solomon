package tools

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureReadFile(path string) {}

type readArgs struct {
	Path string `json:"path"`
}

func readFileOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("readFile", "Read a text file relative to project root.", map[string]any{
		"path": map[string]any{"type": "string", "description": "Path relative to project root"},
	}, []string{"path"})
}

func appendReadFileDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureReadFile)
	if err != nil {
		return err
	}
	b.addBlock("readFile", "Read a text file relative to project root.", sig)
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
	return map[string]any{"path": p, "content": string(b)}, nil
}

func resolveProjectPath(root, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(root, filepath.Clean(p))
}
