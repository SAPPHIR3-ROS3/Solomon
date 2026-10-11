import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, plugins: [{ name: "mock-notification-control", enforce: "pre", resolveId(id) { if (id.endsWith("/notifications/Notifications")) return "virtual:notifications"; }, load(id) { if (id === "virtual:notifications") return "export const NotificationControl = () => null;"; } }], server: { middlewareMode: true, hmr: false }, esbuild: { jsx: "automatic" } });
after(() => server.close());
const { ChatView } = await server.ssrLoadModule("/src/chat/ChatView.tsx");
const first = { id: "user-first", role: "user", content: "First prompt" };
const props = { chat: { id: "chat-1", title: "Test", messages: [first], status: "running" }, isStreaming: true, onSend() {}, onDeleteMessage() {}, onStopTool() {}, onStopStreaming() {} };
function sections(html) {
  const dock = html.indexOf('class="welcome-composer-dock chat-composer-dock"');
  assert.ok(dock > 0);
  return [html.slice(0, dock), html.slice(dock)];
}
test("the first prompt stays in the transcript while the assistant starts", () => {
  const [transcript, composer] = sections(renderToStaticMarkup(createElement(ChatView, props)));
  assert.match(transcript, /First prompt/);
  assert.doesNotMatch(composer, /First prompt|chat-pending-message/);
});
test("queued prompts render in order above the composer and outside the transcript", () => {
  const queuedMessages = [
    { id: "user-second", role: "user", content: "Second prompt" },
    { id: "user-third", role: "user", content: "Third prompt", images: [{ name: "test.png", url: "data:image/png;base64,AAAA" }] },
  ];
  const [transcript, composer] = sections(renderToStaticMarkup(createElement(ChatView, { ...props, queuedMessages })));
  assert.match(transcript, /First prompt/);
  assert.doesNotMatch(transcript, /Second prompt|Third prompt/);
  assert.match(composer, /aria-label="Queued messages" aria-live="polite"/);
  assert.ok(composer.indexOf("Second prompt") < composer.indexOf("Third prompt"));
  assert.ok(composer.indexOf("Third prompt") < composer.indexOf('class="chat-composer"'));
  assert.match(composer, /test.png/);
});
