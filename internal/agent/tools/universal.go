package tools

import "github.com/openai/openai-go/v2"

func IsUniversalTool(name string) bool {
	return name == "docsRetrieval" || name == "readChat"
}

func universalToolParams() []openai.ChatCompletionToolUnionParam {
	return []openai.ChatCompletionToolUnionParam{docsRetrievalOpenAI(), readChatOpenAI()}
}

func EnsureUniversalTools(tools []openai.ChatCompletionToolUnionParam) []openai.ChatCompletionToolUnionParam {
	out := make([]openai.ChatCompletionToolUnionParam, 0, len(tools)+2)
	for _, tool := range universalToolParams() {
		for _, existing := range tools {
			if existing.OfFunction != nil && existing.OfFunction.Function.Name == tool.OfFunction.Function.Name {
				tool = existing
				break
			}
		}
		out = append(out, tool)
	}
	for _, tool := range tools {
		if tool.OfFunction == nil || !IsUniversalTool(tool.OfFunction.Function.Name) {
			out = append(out, tool)
		}
	}
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
