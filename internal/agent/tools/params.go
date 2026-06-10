package tools

import (
	"github.com/openai/openai-go/v2"
)

func NativeToolParams(mode string) ([]openai.ChatCompletionToolUnionParam, error) {
	var tools []openai.ChatCompletionToolUnionParam
	switch normalizeMode(mode) {
	case "agent":
		tools = []openai.ChatCompletionToolUnionParam{
			searchSkillOpenAI(),
			loadSkillOpenAI(),
			searchToolsOpenAI(),
			orchestrateOpenAI(),
			switchModeOpenAI(),
		}
	case "chat":
		tools = []openai.ChatCompletionToolUnionParam{
			fetchWebOpenAI(),
			webSearchOpenAI(),
			switchModeOpenAI(),
		}
	case "plan":
		tools = []openai.ChatCompletionToolUnionParam{
			createPlanOpenAI(),
			editPlanOpenAI(),
			buildPlanOpenAI(),
		}
	case "build":
		tools = []openai.ChatCompletionToolUnionParam{
			shellOpenAI(),
			readFileOpenAI(),
			editFileOpenAI(),
			findOpenAI(),
			subagentOpenAI(),
			loadSkillOpenAI(),
			searchSkillOpenAI(),
			fetchWebOpenAI(),
			webSearchOpenAI(),
		}
	default:
		return universalToolParams(), nil
	}
	return EnsureUniversalTools(tools), nil
}
