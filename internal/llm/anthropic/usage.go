package anthropic

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/apitype"

type UsagePayload struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

func NormalizeUsage(u UsagePayload) apitype.UsageStats {
	prompt := u.InputTokens
	if u.CacheReadInputTokens > 0 {
		prompt += u.CacheReadInputTokens
	}
	if u.CacheCreationInputTokens > 0 {
		prompt += u.CacheCreationInputTokens
	}
	total := prompt + u.OutputTokens
	return apitype.UsageStats{
		PromptTokens:              prompt,
		CachedPromptTokens:        u.CacheReadInputTokens,
		CacheCreationPromptTokens: u.CacheCreationInputTokens,
		ResponseTokens:            u.OutputTokens,
		TotalTokens:               total,
	}
}
