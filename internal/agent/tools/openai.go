package tools

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/shared"
)

func nativeToolUnion(name, desc string, props map[string]any, required []string) openai.ChatCompletionToolUnionParam {
	parameters := tooling.SchemaWithRequiredToolIntent(map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	})
	return openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        name,
				Description: openai.String(desc),
				Parameters:  openai.FunctionParameters(parameters),
			},
		},
	}
}
