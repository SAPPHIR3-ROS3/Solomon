package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/skills"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/openai/openai-go/v2"
)

func signatureInstallSkill(args []string, intent string) {}

func installSkillOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("installSkill", "Install skills using the same installer as /add. Use only when the user requests installation. Accepts skills.sh URLs, npx/npm skills add commands, or skill <markdown path or URL>, with optional name and global/project/local scope (default global).", map[string]any{
		"args": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Arguments after /add, e.g. [\"https://skills.sh/openai/codex/babysit-pr\", \"global\"]"},
	}, []string{"args"})
}

func appendInstallSkillDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureInstallSkill)
	if err != nil {
		return err
	}
	b.addBlock("installSkill", "Install a user-requested skill via the /add installer; args are /add arguments, including optional scope.", sig)
	return nil
}

func execInstallSkill(ctx context.Context, env *Env, raw json.RawMessage) (any, error) {
	var a struct {
		Args []string `json:"args"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	if env == nil || env.ProjRoot == "" || env.ProjHex == "" {
		return nil, fmt.Errorf("installSkill: missing project root or id")
	}
	var out bytes.Buffer
	err := skills.RunInstall(skills.InstallOpts{Ctx: ctx, Out: &out, ProjHex: env.ProjHex, ProjRoot: env.ProjRoot, Args: a.Args, NonInteractive: true})
	if err != nil {
		return nil, fmt.Errorf("installSkill: %w\n%s", err, out.String())
	}
	return map[string]any{"ok": true, "output": out.String()}, nil
}
