package tokcount

import (
	"encoding/json"
	"strings"
)

func MessageOverhead(model string) (tokensPerMessage, tokensPerName int) {
	return messageOverhead(model)
}

func messageOverhead(model string) (tokensPerMessage, tokensPerName int) {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return 3, 1
	}
	if strings.Contains(m, "gpt-3.5-turbo-0301") {
		return 4, -1
	}
	if strings.Contains(m, "gpt-3.5-turbo") || strings.Contains(m, "gpt-4") || strings.Contains(m, "gpt-4o") || strings.Contains(m, "gpt-5") || strings.Contains(m, "o1") || strings.Contains(m, "o3") || strings.Contains(m, "o4") {
		return 3, 1
	}
	return 3, 1
}

func encodeString(tkm interface{ Encode(string, []string, []string) []int }, s string) int64 {
	if s == "" {
		return 0
	}
	return int64(len(tkm.Encode(s, nil, nil)))
}

func encodeJSONField(tkm interface{ Encode(string, []string, []string) []int }, raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var asString string
	if json.Unmarshal(raw, &asString) == nil {
		return encodeString(tkm, asString)
	}
	return encodeString(tkm, string(raw))
}

func countMessageMap(tkm interface{ Encode(string, []string, []string) []int }, raw map[string]json.RawMessage, tokensPerName int) int64 {
	var n int64
	for key, val := range raw {
		n += encodeJSONField(tkm, val)
		if key == "name" {
			n += int64(tokensPerName)
		}
	}
	return n
}
