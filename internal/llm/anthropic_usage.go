package llm

type AnthropicUsagePayload struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

func NormalizeAnthropicUsage(u AnthropicUsagePayload) UsageStats {
	prompt := u.InputTokens + u.CacheReadInputTokens
	total := prompt + u.OutputTokens
	return UsageStats{
		PromptTokens:              prompt,
		CachedPromptTokens:        u.CacheReadInputTokens,
		CacheCreationPromptTokens: u.CacheCreationInputTokens,
		ResponseTokens:            u.OutputTokens,
		TotalTokens:               total,
	}
}
