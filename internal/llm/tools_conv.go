package llm

import (
	"encoding/json"

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
		var required []string
		if fn.Parameters != nil {
			raw, _ := json.Marshal(fn.Parameters)
			_ = json.Unmarshal(raw, &params)
			if r, ok := params["required"].([]any); ok {
				for _, x := range r {
					if s, ok := x.(string); ok {
						required = append(required, s)
					}
				}
			} else if rs, ok := params["required"].([]string); ok {
				required = rs
			}
		}
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
