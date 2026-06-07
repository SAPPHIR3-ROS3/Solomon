package tools

import (
	"fmt"

	"github.com/openai/openai-go/v2"
)

func NativeToolParams(mode string) ([]openai.ChatCompletionToolUnionParam, error) {
	switch mode {
	case "plan":
		return []openai.ChatCompletionToolUnionParam{
			docsRetrievalOpenAI(),
			createPlanOpenAI(),
			editPlanOpenAI(),
			buildPlanOpenAI(),
		}, nil
	case "build":
		return []openai.ChatCompletionToolUnionParam{
			docsRetrievalOpenAI(),
			shellOpenAI(),
			readFileOpenAI(),
			editFileOpenAI(),
			findOpenAI(),
			subagentOpenAI(),
			loadSkillOpenAI(),
			searchSkillOpenAI(),
			fetchWebOpenAI(),
			webSearchOpenAI(),
		}, nil
	default:
		return nil, fmt.Errorf("unknown mode %q", mode)
	}
}
