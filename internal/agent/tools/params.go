package tools

import (
	"github.com/openai/openai-go/v2"
)

func NativeToolParams(mode string) ([]openai.ChatCompletionToolUnionParam, error) {
	var tools []openai.ChatCompletionToolUnionParam
	switch normalizeMode(mode) {
	case "chat":
		tools = []openai.ChatCompletionToolUnionParam{
			fetchWebOpenAI(),
			webSearchOpenAI(),
			deepResearchOpenAI(),
			researchStatusOpenAI(),
			switchModeOpenAI(),
		}
	default:
		tools = []openai.ChatCompletionToolUnionParam{
			searchSkillOpenAI(),
			loadSkillOpenAI(),
			searchToolsOpenAI(),
			orchestrateOpenAI(),
			subagentOpenAI(),
			listSubAgentsOpenAI(),
			switchModeOpenAI(),
		}
	}
	return EnsureUniversalTools(tools), nil
}
