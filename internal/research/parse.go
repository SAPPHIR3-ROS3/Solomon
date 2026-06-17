package research

import (
	"encoding/json"
	"regexp"
	"strings"
)

var codeBlockRe = regexp.MustCompile("(?s)```(?:json)?\\s*|\\s*```")

func stripCodeBlock(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		text = codeBlockRe.ReplaceAllString(text, "")
	}
	return strings.TrimSpace(text)
}

func parseJSONArray(text string) []string {
	text = stripCodeBlock(text)
	var parsed []any
	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		out := make([]string, 0, len(parsed))
		for _, item := range parsed {
			out = append(out, strings.TrimSpace(stringifyJSON(item)))
		}
		return out
	}
	re := regexp.MustCompile(`\[[\s\S]*\]`)
	if m := re.FindString(text); m != "" {
		if err := json.Unmarshal([]byte(m), &parsed); err == nil {
			out := make([]string, 0, len(parsed))
			for _, item := range parsed {
				out = append(out, strings.TrimSpace(stringifyJSON(item)))
			}
			return out
		}
	}
	reItems := regexp.MustCompile(`"([^"]*)"`)
	if i := strings.Index(text, "["); i >= 0 {
		if items := reItems.FindAllStringSubmatch(text[i:], -1); len(items) > 0 {
			out := make([]string, 0, len(items))
			for _, m := range items {
				if len(m) > 1 && m[1] != "" {
					out = append(out, m[1])
				}
			}
			return out
		}
	}
	return nil
}

func parseJSONObject(text string) map[string]any {
	text = stripCodeBlock(text)
	var obj map[string]any
	if err := json.Unmarshal([]byte(text), &obj); err == nil {
		return obj
	}
	re := regexp.MustCompile(`\{[\s\S]*\}`)
	if m := re.FindString(text); m != "" {
		if err := json.Unmarshal([]byte(m), &obj); err == nil {
			return obj
		}
	}
	return nil
}

func stringifyJSON(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

func planFromJSON(obj map[string]any) string {
	if obj == nil {
		return ""
	}
	var parts []string
	if sq, ok := obj["sub_questions"].([]any); ok && len(sq) > 0 {
		items := make([]string, 0, len(sq))
		for _, x := range sq {
			items = append(items, stringifyJSON(x))
		}
		parts = append(parts, "Sub-questions: "+strings.Join(items, "; "))
	}
	if kt, ok := obj["key_topics"].([]any); ok && len(kt) > 0 {
		items := make([]string, 0, len(kt))
		for _, x := range kt {
			items = append(items, stringifyJSON(x))
		}
		parts = append(parts, "Key topics: "+strings.Join(items, ", "))
	}
	if sc, ok := obj["success_criteria"].(string); ok && sc != "" {
		parts = append(parts, "Success: "+sc)
	}
	return strings.Join(parts, "\n")
}

func shouldStopFromLLM(response string) bool {
	clean := strings.TrimSpace(response)
	clean = regexp.MustCompile(`^[\s*_` + "`" + `"'>#\-]+`).ReplaceAllString(clean, "")
	return strings.HasPrefix(strings.ToUpper(clean), "YES")
}

func normalizeCategory(raw string) string {
	cat := strings.ToLower(strings.TrimSpace(raw))
	if cat == "" || cat == "general" {
		return ""
	}
	first := cat
	if i := strings.IndexAny(cat, " \t\n"); i >= 0 {
		first = cat[:i]
	}
	first = strings.Trim(first, ".,\"'*:")
	if _, ok := categoryOverrides[first]; ok {
		return first
	}
	for k := range categoryOverrides {
		if strings.Contains(cat, k) {
			return k
		}
	}
	return ""
}
