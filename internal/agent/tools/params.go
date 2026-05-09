package tools

import (
	"fmt"

	"github.com/openai/openai-go/v2"
)

func NativeToolParams(mode string) ([]openai.ChatCompletionToolUnionParam, error) {
	switch mode {
	case "plan":
		return []openai.ChatCompletionToolUnionParam{
			createPlanOpenAI(),
			editPlanOpenAI(),
			buildPlanOpenAI(),
		}, nil
	case "build":
		return []openai.ChatCompletionToolUnionParam{
			shellOpenAI(),
			readFileOpenAI(),
			editFileOpenAI(),
			subagentOpenAI(),
			loadSkillOpenAI(),
			searchSkillOpenAI(),
		}, nil
	default:
		return nil, fmt.Errorf("unknown mode %q", mode)
	}
}
