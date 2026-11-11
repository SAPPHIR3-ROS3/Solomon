import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { liveChatFromPayload } = await server.ssrLoadModule("/src/chat/chatClient.ts");
const { assistantFooterMessage, indexChatMessages } = await server.ssrLoadModule("/src/chat/chatMessageUtils.ts");

test("history preserves recorded message times through assistant grouping", () => {
  const dates = ["2026-10-20T13:00:00+02:00", "2026-10-20T13:01:00+02:00", "2026-10-20T13:02:00+02:00"];
  const chat = liveChatFromPayload({ id: "chat", messages: dates.map((createdAt, index) => ({ id: `m-${index}`, role: index ? "assistant" : "user", content: "text", createdAt })) }, "project");
  assert.deepEqual(chat.messages.map(message => message.createdAt), dates.map(Date.parse));
  assert.equal(assistantFooterMessage(indexChatMessages(chat.messages.slice(1))).createdAt, Date.parse(dates[2]));
});

test("message info shows the latest request provider and model when grouping a turn", () => {
  const messages = [
    { provider: "provider-a", model: "model-a" },
    { provider: "provider-a", model: "model-a" },
    { provider: "provider-b", model: "model-b" },
  ].map((stats, index) => ({ id: `a${index}`, role: "assistant", content: "answer", stats }));
  const chat = liveChatFromPayload({ id: "chat", messages }, "project");
  assert.equal(chat.messages[0].stats.provider, "provider-a");
  assert.equal(chat.messages[0].stats.model, "model-a");
  const footer = assistantFooterMessage(indexChatMessages(chat.messages));
  assert.equal(footer.stats.provider, "provider-b");
  assert.equal(footer.stats.model, "model-b");
});

test("legacy statistics keep unknown provider and model unset", () => {
  const chat = liveChatFromPayload({ id: "chat", messages: [{ id: "a", role: "assistant", content: "old answer", stats: { responseTokens: 5 } }] }, "project");
  assert.equal(chat.messages[0].stats.provider, undefined);
  assert.equal(chat.messages[0].stats.model, undefined);
  assert.equal(chat.messages[0].stats.responseTokens, 5);
});
