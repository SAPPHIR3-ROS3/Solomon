package tools

import "github.com/openai/openai-go/v2"

func PlanningNativeToolParams() []openai.ChatCompletionToolUnionParam {
	return []openai.ChatCompletionToolUnionParam{
		buildPlanOpenAI(),
	}
}
