package tools

import "github.com/openai/openai-go/v2"

func PlanNativeToolParams() []openai.ChatCompletionToolUnionParam {
	return []openai.ChatCompletionToolUnionParam{
		createPlanOpenAI(),
		editPlanOpenAI(),
		buildPlanOpenAI(),
		addTodoOpenAI(),
		todoListOpenAI(),
		checkTodoOpenAI(),
		removeTodoOpenAI(),
		checkPlanOpenAI(),
		deletePlanOpenAI(),
	}
}
