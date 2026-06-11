package tokcount

import (
	"encoding/json"
	"strings"

	"github.com/openai/openai-go/v2"
)

func toolSchemaOverhead(model string) (funcInit, propInit, propKey, enumInit, enumItem, funcEnd int) {
	m := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(m, "gpt-4o"), strings.Contains(m, "gpt-5"), strings.Contains(m, "o1"), strings.Contains(m, "o3"), strings.Contains(m, "o4"):
		return 7, 3, 3, -3, 3, 12
	default:
		return 10, 3, 3, -3, 3, 12
	}
}

func CountTools(tools []openai.ChatCompletionToolUnionParam, model string) int64 {
	if len(tools) == 0 {
		return 0
	}
	tkm, err := EncoderForModel(model)
	if err != nil {
		return 0
	}
	funcInit, propInit, propKey, enumInit, enumItem, funcEnd := toolSchemaOverhead(model)
	var total int64
	for _, t := range tools {
		if t.OfFunction == nil {
			continue
		}
		fn := t.OfFunction.Function
		total += int64(funcInit)
		name := fn.Name
		desc := ""
		if fn.Description.Valid() {
			desc = strings.TrimSpace(fn.Description.Value)
		}
		if strings.HasSuffix(desc, ".") {
			desc = strings.TrimSuffix(desc, ".")
		}
		total += encodeString(tkm, name+":"+desc)
		props := toolProperties(fn.Parameters)
		if len(props) == 0 {
			total += int64(funcEnd)
			continue
		}
		total += int64(propInit)
		for key, spec := range props {
			total += int64(propKey)
			pType, _ := spec["type"].(string)
			pDesc, _ := spec["description"].(string)
			if strings.HasSuffix(pDesc, ".") {
				pDesc = strings.TrimSuffix(pDesc, ".")
			}
			if enumVals, ok := spec["enum"].([]any); ok && len(enumVals) > 0 {
				total += int64(enumInit)
				for _, item := range enumVals {
					if s, ok := item.(string); ok {
						total += int64(enumItem)
						total += encodeString(tkm, s)
					}
				}
			}
			total += encodeString(tkm, key+":"+pType+":"+pDesc)
		}
		total += int64(funcEnd)
	}
	return total
}

func toolProperties(params openai.FunctionParameters) map[string]map[string]any {
	if params == nil {
		return nil
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return nil
	}
	var root map[string]any
	if json.Unmarshal(raw, &root) != nil {
		return nil
	}
	props, _ := root["properties"].(map[string]any)
	if props == nil {
		return nil
	}
	out := make(map[string]map[string]any, len(props))
	for k, v := range props {
		m, _ := v.(map[string]any)
		if m != nil {
			out[k] = m
		}
	}
	return out
}
