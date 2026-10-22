package stream

import (
	"encoding/json"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/apitype"
	"github.com/openai/openai-go/v2"
	"testing"
)

func TestCacheCounterPresence(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int64
	}{
		{`{}`, -1}, {`{"prompt_tokens_details":{"cached_tokens":0}}`, 0},
		{`{"input_tokens_details":{"cached_tokens":123}}`, 123},
		{`{"cached_tokens":null}`, -1}, {`{"cached_tokens":"bad"}`, -1},
		{`{"cache_read_input_tokens":42}`, 42},
	} {
		if got := parseCacheCounter(tc.raw, "cached_tokens", "cache_read_input_tokens"); got != tc.want {
			t.Errorf("%s: got %d want %d", tc.raw, got, tc.want)
		}
	}
	for _, tc := range []struct {
		raw  string
		want int64
	}{
		{`{}`, -1}, {`{"cache_creation_input_tokens":0}`, 0},
		{`{"prompt_tokens_details":{"cache_write_tokens":12}}`, 12},
	} {
		if got := parseLooseCacheWriteTokensFromUsageRawJSON(tc.raw); got != tc.want {
			t.Errorf("%s: got %d want %d", tc.raw, got, tc.want)
		}
	}
}

func TestAggregateCacheUnknown(t *testing.T) {
	u := AggregateConsecutiveTurnUsage([]apitype.UsageStats{
		{CachedPromptTokens: 12, CacheCreationPromptTokens: 0},
		{CachedPromptTokens: -1, CacheCreationPromptTokens: 0},
	})
	if u.CachedPromptTokens != -1 || u.CacheCreationPromptTokens != 0 {
		t.Fatalf("unexpected aggregate: %+v", u)
	}
}

func TestCacheUsageSurvivesAccumulator(t *testing.T) {
	for _, raw := range []string{
		`{"prompt_tokens":100,"prompt_tokens_details":{"cached_tokens":0}}`,
		`{"prompt_tokens":100,"prompt_tokens_details":{"cached_tokens":72},"cache_creation_input_tokens":8}`,
		`{"prompt_tokens":100}`,
	} {
		var chunk openai.ChatCompletionChunk
		if err := json.Unmarshal([]byte(`{"id":"test","object":"chat.completion.chunk","choices":[],"usage":`+raw+`}`), &chunk); err != nil {
			t.Fatal(err)
		}
		var acc openai.ChatCompletionAccumulator
		acc.AddChunk(chunk)
		got := usageFromAccumulator(acc, 0, chunk.Usage.RawJSON())
		read := parseCacheCounter(raw, "cached_tokens")
		write := parseLooseCacheWriteTokensFromUsageRawJSON(raw)
		if got.CachedPromptTokens != read || got.CacheCreationPromptTokens != write {
			t.Fatalf("usage lost through SDK accumulator: %+v, raw %s", got, raw)
		}
	}
}
