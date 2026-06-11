package tokcount

import (
	"encoding/json"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/param"
)

func CountOpenAIMessages(msgs []openai.ChatCompletionMessageParamUnion, model string) int64 {
	if len(msgs) == 0 {
		return 0
	}
	tkm, err := EncoderForModel(model)
	if err != nil {
		return 0
	}
	tpm, tpn := messageOverhead(model)
	var total int64
	for _, m := range msgs {
		total += int64(tpm)
		total += countMessageUnion(tkm, m, tpn)
	}
	total += 3
	return total
}

func countMessageUnion(tkm interface{ Encode(string, []string, []string) []int }, m openai.ChatCompletionMessageParamUnion, tokensPerName int) int64 {
	raw, err := json.Marshal(m)
	if err != nil {
		return 0
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return 0
	}
	var n int64
	for key, val := range fields {
		switch key {
		case "content":
			n += countContentField(tkm, val)
		case "tool_calls":
			n += encodeJSONField(tkm, val)
		default:
			n += encodeJSONField(tkm, val)
		}
		if key == "name" {
			n += int64(tokensPerName)
		}
	}
	return n
}

func countContentField(tkm interface{ Encode(string, []string, []string) []int }, raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var asString string
	if json.Unmarshal(raw, &asString) == nil {
		return encodeString(tkm, asString)
	}
	var parts []map[string]json.RawMessage
	if json.Unmarshal(raw, &parts) != nil {
		return encodeString(tkm, string(raw))
	}
	var n int64
	for _, part := range parts {
		if textRaw, ok := part["text"]; ok {
			n += encodeJSONField(tkm, textRaw)
			continue
		}
		if urlRaw, ok := part["image_url"]; ok {
			var wrap struct {
				URL string `json:"url"`
			}
			if json.Unmarshal(urlRaw, &wrap) == nil && wrap.URL != "" {
				n += VisionTokensForDataURL(wrap.URL)
			}
			continue
		}
		n += encodeJSONField(tkm, mustJSON(part))
	}
	return n
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func CountOpenAIChatCompletion(msgs []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolUnionParam, model string) int64 {
	return CountOpenAIMessages(msgs, model) + CountTools(tools, model)
}

func CountOpenAIChatCompletionParams(p openai.ChatCompletionNewParams) int64 {
	model := string(p.Model)
	if param.IsOmitted(p.Model) {
		model = DefaultModel
	}
	return CountOpenAIChatCompletion(p.Messages, p.Tools, model)
}
