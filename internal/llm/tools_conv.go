package llm

import (
	"encoding/json"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/openai/openai-go/v2"
)

func ToolDefsFromOpenAI(tools []openai.ChatCompletionToolUnionParam) []ToolDef {
	var out []ToolDef
	for _, t := range tools {
		if t.OfFunction == nil {
			continue
		}
		fn := t.OfFunction.Function
		params := map[string]any{}
		if fn.Parameters != nil {
			raw, _ := json.Marshal(fn.Parameters)
			_ = json.Unmarshal(raw, &params)
		}
		params = tooling.SchemaWithRequiredToolIntent(params)
		required := requiredToolDefNames(params)
		desc := ""
		if fn.Description.Valid() {
			desc = fn.Description.Value
		}
		out = append(out, ToolDef{
			Name:        fn.Name,
			Description: desc,
			Parameters:  params,
			Required:    required,
		})
	}
	return out
}

func requiredToolDefNames(params map[string]any) []string {
	var required []string
	if values, ok := params["required"].([]any); ok {
		for _, value := range values {
			if name, ok := value.(string); ok {
				required = append(required, name)
			}
		}
	}
	return required
}
