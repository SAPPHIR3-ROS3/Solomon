import assert from "node:assert/strict";
import { test } from "node:test";
import { buildOpenAIUsage } from "../src/chat/helpers/usage.js";

test("Cursor bridge preserves explicit cache zeros and omits unknown counters", () => {
 const missing = buildOpenAIUsage([], undefined, "", "");
 assert.equal(missing.prompt_tokens_details, undefined);
 assert.equal(missing.cache_creation_input_tokens, undefined);
 const zero = buildOpenAIUsage([], {inputTokens: 10, outputTokens: 2, cacheReadTokens: 0, cacheWriteTokens: 0}, "", "");
 assert.equal(zero.prompt_tokens_details?.cached_tokens, 0);
 assert.equal(zero.cache_creation_input_tokens, 0);
 const positive = buildOpenAIUsage([], {inputTokens: 10, outputTokens: 2, cacheReadTokens: 8, cacheWriteTokens: 1}, "", "");
 assert.equal(positive.prompt_tokens_details?.cached_tokens, 8);
 assert.equal(positive.cache_creation_input_tokens, 1);
});
