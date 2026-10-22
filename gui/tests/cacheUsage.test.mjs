import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { assistantFooterMessage, indexChatMessages } = await server.ssrLoadModule("/src/chat/chatMessageUtils.ts");
const footer = (stats) => assistantFooterMessage(indexChatMessages(stats.map((stats, i) => ({ id: String(i), role: "assistant", content: "", stats })))).stats;
test("cache totals distinguish reported zero, missing values and incomplete turns", () => {
 assert.equal(footer([{cachedPromptTokens: 0}, {cachedPromptTokens: 0}]).cachedPromptTokens, 0);
 assert.equal(footer([{cachedPromptTokens: 12}, {cachedPromptTokens: -1}]).cachedPromptTokens, -1);
 assert.equal(footer([{cachedPromptTokens: 12}, {}]).cachedPromptTokens, -1);
 assert.equal(footer([{cachedPromptTokens: 12}, {cachedPromptTokens: 8}]).cachedPromptTokens, 20);
 assert.equal(footer([{cacheCreationPromptTokens: 2}, {}]).cacheCreationPromptTokens, -1);
 assert.equal(footer([{cachedPromptTokens: 12}, {turnAggregate: true, cachedPromptTokens: -1}]).cachedPromptTokens, -1);
});
