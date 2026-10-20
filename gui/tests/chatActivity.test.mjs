import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { applyProjectChatActivity, recordProjectChatActivity, PROJECT_CHAT_ACTIVITY_EVENT } = await server.ssrLoadModule("/src/projects/chatActivity.ts");

test("sending immediately promotes the chat and its project, including across stale refreshes", (t) => {
  const originalWindow = globalThis.window;
  globalThis.window = new EventTarget();
  t.after(() => { globalThis.window = originalWindow; });
  t.mock.method(Date, "now", () => Date.parse("2026-10-20T12:00:00Z"));
  const summary = (id, lastMessageAt) => ({ id, title: id, lastMessageAt });
  const projects = [
    { id: "recent", chats: [summary("recent-chat", "2026-10-20T11:00:00Z")], chatCount: 1 },
    { id: "older", chats: [summary("other-chat", "2026-10-19T11:00:00Z"), summary("sent-chat", "2026-10-18T11:00:00Z")], chatCount: 2 },
  ];
  let visible = projects;
  let notifications = 0;
  window.addEventListener(PROJECT_CHAT_ACTIVITY_EVENT, () => {
    notifications++;
    visible = applyProjectChatActivity(visible);
  });

  recordProjectChatActivity("older", "sent-chat");
  assert.equal(notifications, 1);
  assert.equal(visible[0].id, "older");
  const sortedChats = [...visible[0].chats].sort((a, b) => Date.parse(b.lastMessageAt) - Date.parse(a.lastMessageAt));
  assert.equal(sortedChats[0].id, "sent-chat");
  assert.deepEqual(applyProjectChatActivity(projects), visible, "an older server response must preserve local send activity");
  assert.equal(projects[1].chats[1].lastMessageAt, "2026-10-18T11:00:00Z", "do not mutate cached server data");

  // A further send while the response is running must also update activity.
  t.mock.method(Date, "now", () => Date.parse("2026-10-20T13:00:00Z"));
  recordProjectChatActivity("older", "sent-chat");
  assert.equal(notifications, 2);
  assert.equal(visible[0].chats[1].lastMessageAt, "2026-10-20T13:00:00.000Z");

  // A later send in another chat wins even when the first reply finishes last.
  t.mock.method(Date, "now", () => Date.parse("2026-10-20T14:00:00Z"));
  recordProjectChatActivity("recent", "recent-chat");
  const completed = [projects[0], { ...projects[1], chats: [projects[1].chats[0], summary("sent-chat", "2026-10-20T15:00:00Z")] }];
  const afterCompletion = applyProjectChatActivity(completed);
  assert.equal(afterCompletion[0].id, "recent", "completion must not promote the earlier chat's folder");
  assert.equal(afterCompletion[1].chats[1].lastMessageAt, "2026-10-20T13:00:00.000Z", "completion must not change the send timestamp");

  t.mock.method(Date, "now", () => Date.parse("2026-10-20T16:00:00Z"));
  recordProjectChatActivity("older", "other-chat");
  const afterNextSend = applyProjectChatActivity(completed);
  assert.equal(afterNextSend[0].id, "older", "a new send promotes the folder again");
  assert.equal(afterNextSend[0].chats[0].lastMessageAt, "2026-10-20T16:00:00.000Z");
  const deleted = [{ ...projects[1], chats: [], chatCount: 0 }];
  assert.equal(applyProjectChatActivity(deleted)[0].chats.length, 0, "local activity must not resurrect deleted chats");
});
