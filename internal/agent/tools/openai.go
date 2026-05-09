package tools

import (
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
