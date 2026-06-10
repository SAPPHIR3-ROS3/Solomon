package tools

import "github.com/openai/openai-go/v2"

func IsUniversalTool(name string) bool {
	return name == "docsRetrieval"
}

func universalToolParams() []openai.ChatCompletionToolUnionParam {
	return []openai.ChatCompletionToolUnionParam{docsRetrievalOpenAI()}
}

func EnsureUniversalTools(tools []openai.ChatCompletionToolUnionParam) []openai.ChatCompletionToolUnionParam {
	if toolParamsHasName(tools, "docsRetrieval") {
		return tools
	}
	out := make([]openai.ChatCompletionToolUnionParam, 0, len(tools)+1)
	out = append(out, docsRetrievalOpenAI())
	out = append(out, tools...)
	return out
}

func toolParamsHasName(tools []openai.ChatCompletionToolUnionParam, name string) bool {
	for _, t := range tools {
		if t.OfFunction != nil && t.OfFunction.Function.Name == name {
			return true
		}
	}
	return false
}
