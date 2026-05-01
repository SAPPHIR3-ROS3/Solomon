package agent

import (
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooling"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/shared"
)

func nativeToolUnion(name, desc string, props map[string]any, required []string) openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        name,
				Description: openai.String(desc),
				Parameters: openai.FunctionParameters{
					"type":                 "object",
					"properties":           props,
					"required":             required,
					"additionalProperties": false,
				},
			},
		},
	}
}

func NativeToolParams(mode string) ([]openai.ChatCompletionToolUnionParam, error) {
	switch mode {
	case "plan":
		return []openai.ChatCompletionToolUnionParam{
			nativeToolUnion("createPlan", "Create or overwrite a plan file (markdown) under the project plans directory.", map[string]any{
				"name":     map[string]any{"type": "string", "description": "Plan filename, e.g. feature.md"},
				"planText": map[string]any{"type": "string", "description": "Full markdown body for the plan"},
			}, []string{"name", "planText"}),
			nativeToolUnion("editPlan", "Replace first occurrence of old segment in plan file.", map[string]any{
				"name": map[string]any{"type": "string", "description": "Plan filename"},
				"old":  map[string]any{"type": "string", "description": "Exact substring to replace once"},
				"new":  map[string]any{"type": "string", "description": "Replacement text"},
			}, []string{"name", "old", "new"}),
			nativeToolUnion("buildPlan", "Switch to BUILD mode and run an implementation session for the named plan.", map[string]any{
				"name": map[string]any{"type": "string", "description": "Plan filename to implement"},
			}, []string{"name"}),
		}, nil
	case "build":
		return []openai.ChatCompletionToolUnionParam{
			nativeToolUnion("shell", "Run a shell command in the harness working directory.", map[string]any{
				"command": map[string]any{"type": "string", "description": "Shell command to run"},
				"timeoutSeconds": map[string]any{
					"type":        "integer",
					"description": "Optional timeout in seconds for this command",
				},
			}, []string{"command"}),
			nativeToolUnion("readFile", "Read a text file relative to project root.", map[string]any{
				"path": map[string]any{"type": "string", "description": "Path relative to project root"},
			}, []string{"path"}),
			nativeToolUnion("editFile", "Replace oldString once with newString, or write newString when oldString is empty.", map[string]any{
				"path":      map[string]any{"type": "string", "description": "Path relative to project root"},
				"oldString": map[string]any{"type": "string", "description": "Substring to replace once; empty means create/overwrite per tool semantics"},
				"newString": map[string]any{"type": "string", "description": "New content or replacement text"},
			}, []string{"path", "oldString", "newString"}),
			nativeToolUnion("subagent", "Run a nested agent with system prompt from file and task string.", map[string]any{
				"sysPromptPath": map[string]any{"type": "string", "description": "Path to system prompt file"},
				"task":          map[string]any{"type": "string", "description": "Concrete task for the nested run"},
			}, []string{"sysPromptPath", "task"}),
			nativeToolUnion("loadSkill", "Load installed agent skill body (markdown body only). Name is the display name or slash token without leading slash.", map[string]any{
				"name": map[string]any{"type": "string", "description": "Skill display name or slash command token (e.g. my-skill)"},
			}, []string{"name"}),
			nativeToolUnion("searchSkill", "BM25 search over installed skills; returns one best match or an error if nothing passes the quality threshold. Tries descriptions first, then full SKILL.md. The returned score is normalized to [0,1] (best raw BM25 for the query divided by a corpus ceiling). Default minimum is 0.05 (config key skill_search_min_normalized_score; set to 0 to disable).", map[string]any{
				"query": map[string]any{"type": "string", "description": "Search query"},
			}, []string{"query"}),
		}, nil
	default:
		return nil, fmt.Errorf("unknown mode %q", mode)
	}
}

func buildPlanToolDump() (string, error) {
	b := &dumpBuilder{}
	sig, err := tooling.FuncSignature(signatureCreatePlan)
	if err != nil {
		return "", err
	}
	b.addBlock("createPlan", "Create or overwrite a plan file (markdown) under the project plans directory.", sig)
	sig, err = tooling.FuncSignature(signatureEditPlan)
	if err != nil {
		return "", err
	}
	b.addBlock("editPlan", "Replace first occurrence of old segment in plan file.", sig)
	sig, err = tooling.FuncSignature(signatureBuildPlan)
	if err != nil {
		return "", err
	}
	b.addBlock("buildPlan", "Switch to BUILD mode and run an implementation session for the named plan.", sig)
	return b.String(), nil
}

func buildBuildToolDump() (string, error) {
	b := &dumpBuilder{}
	sig, err := tooling.FuncSignature(signatureShell)
	if err != nil {
		return "", err
	}
	b.addBlock("shell", "Run a shell command in the harness working directory. Optional JSON fields may tweak behavior.", sig)
	sig, err = tooling.FuncSignature(signatureReadFile)
	if err != nil {
		return "", err
	}
	b.addBlock("readFile", "Read a text file relative to project root.", sig)
	sig, err = tooling.FuncSignature(signatureEditFile)
	if err != nil {
		return "", err
	}
	b.addBlock("editFile", "Replace oldString once with newString, or write newString when oldString empty.", sig)
	sig, err = tooling.FuncSignature(signatureSubagent)
	if err != nil {
		return "", err
	}
	b.addBlock("subagent", "Run a nested agent with system prompt from file and task string.", sig)
	sig, err = tooling.FuncSignature(signatureLoadSkill)
	if err != nil {
		return "", err
	}
	b.addBlock("loadSkill", "Load installed agent skill body (markdown body only).", sig)
	sig, err = tooling.FuncSignature(signatureSearchSkill)
	if err != nil {
		return "", err
	}
	b.addBlock("searchSkill", "BM25 search; two phases; score normalized [0,1]; min threshold from config (default 0.05).", sig)
	return b.String(), nil
}

type dumpBuilder struct {
	s string
}

func (b *dumpBuilder) addBlock(name, desc, sig string) {
	if b.s != "" {
		b.s += "\n---\n"
	}
	b.s += fmt.Sprintf("name: %s\ndescription: %s\nsignature: %s\n", name, desc, sig)
}

func (b *dumpBuilder) String() string { return b.s }
