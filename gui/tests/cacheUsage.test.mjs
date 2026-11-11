import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { assistantFooterMessage, indexChatMessages } = await server.ssrLoadModule("/src/chat/chatMessageUtils.ts");
const footer = (stats) => assistantFooterMessage(indexChatMessages(stats.map((stats, i) => ({ id: String(i), role: "assistant", content: "", stats })))).stats;
test("all statistics refer to the latest request, including reported zero and unknown cache", () => {
 const previous = {contextTokens: 177974, cachedPromptTokens: 177792, reasoningTokens: 163, responseTokens: 1155, totalTokens: 179292, outputTokensPerSecond: 10, promptTokensPerSecond: 100, ttftSeconds: 1};
 const latest = {contextTokens: 180689, cachedPromptTokens: 180480, cacheCreationPromptTokens: 0, reasoningTokens: 0, responseTokens: 53, totalTokens: 180742, outputTokensPerSecond: 30, promptTokensPerSecond: 300, ttftSeconds: 2};
 assert.deepEqual(footer([previous, latest]), latest);
 assert.deepEqual(footer([previous, {...latest, cachedPromptTokens: 0}]), {...latest, cachedPromptTokens: 0});
 assert.deepEqual(footer([previous, {...latest, cachedPromptTokens: -1}]), {...latest, cachedPromptTokens: -1});
 assert.deepEqual(footer([previous, {...latest, turnAggregate: false}]), {...latest, turnAggregate: false});
});
