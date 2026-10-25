import assert from "node:assert/strict";
import test from "node:test";
import type { Action, ChatMessage } from "../src/history.ts";
import { ClaudeRunner, type PoolSession } from "../src/runner.ts";

type Fake = PoolSession & { closed: boolean; actions: Action[] };

function pool(opts: { gate?: () => Promise<void>; admitTimeoutMs?: number } = {}) {
  const created: Fake[] = [];
  const runner = new ClaudeRunner("", {
    admitTimeoutMs: opts.admitTimeoutMs,
    createSession(model) {
      let waiting = false;
      const session: Fake = {
        model, accepted: [], produced: null, busy: false, used: Date.now(), closed: false, actions: [],
        get awaitingTools() { return waiting; },
        async run(action) {
          session.actions.push(action);
          if (opts.gate) await opts.gate();
          const prompt = action.type === "start" || action.type === "user" ? action.prompt : "";
          waiting = prompt.startsWith("tool");
          if (waiting) return { content: "", toolCalls: [{ id: "call-" + created.indexOf(session), name: "search", arguments: "{}" }], finishReason: "tool_calls", usage: { promptTokens: 0, completionTokens: 0 } };
          return { content: "done", toolCalls: [], finishReason: "stop", usage: { promptTokens: 0, completionTokens: 0 } };
        },
        async close() { session.closed = true; },
      };
      created.push(session);
      return session;
    },
  });
  return { runner, created };
}

const signal = new AbortController().signal;
const ask = (runner: ClaudeRunner, messages: ChatMessage[]) => runner.complete({ model: "m", messages, tools: [] }, () => {}, signal);

test("a full pool keeps conversations waiting on tool results and still completes them", async () => {
  const { runner, created } = pool();
  const chat: ChatMessage[] = [{ role: "user", content: "tool please" }];
  const first = await ask(runner, chat);
  for (let index = 1; index < 8; index++) await ask(runner, [{ role: "user", content: "chat " + index }]);
  await ask(runner, [{ role: "user", content: "title" }]);
  assert.equal(created[0].closed, false, "the tool-waiting session was evicted");
  assert.equal(created[1].closed, true, "the oldest idle session should make room");
  await ask(runner, [...chat, { role: "assistant", content: "", tool_calls: first.toolCalls }, { role: "tool", tool_call_id: first.toolCalls[0].id, content: "found" }]);
  assert.deepEqual(created[0].actions.at(-1), { type: "tools", results: [{ id: "call-0", output: "found" }] });
  assert.equal(created.filter((session) => !session.closed).length, 8);
});

test("concurrent requests beyond capacity wait for a free slot instead of growing the pool", async () => {
  const releases: Array<() => void> = [];
  const { runner, created } = pool({ gate: () => new Promise<void>((resolve) => releases.push(resolve)) });
  const running = Array.from({ length: 10 }, (_, index) => ask(runner, [{ role: "user", content: "chat " + index }]));
  await new Promise((resolve) => setTimeout(resolve, 10));
  assert.equal(created.length, 8);
  releases.shift()!();
  await new Promise((resolve) => setTimeout(resolve, 10));
  assert.equal(created.filter((session) => !session.closed).length, 8);
  assert.equal(created.length, 9);
  while (releases.length) {
    releases.shift()!();
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  await Promise.all(running);
  assert.ok(created.filter((session) => !session.closed).length <= 8);
});

test("a request fails with a capacity error when every session stays busy", async () => {
  const { runner } = pool({ gate: () => new Promise<void>(() => {}), admitTimeoutMs: 20 });
  for (let index = 0; index < 8; index++) void ask(runner, [{ role: "user", content: "chat " + index }]);
  await assert.rejects(ask(runner, [{ role: "user", content: "title" }]), /at capacity/);
});

test("a cancelled request stops waiting for a slot at once", async () => {
  const { runner } = pool({ gate: () => new Promise<void>(() => {}) });
  for (let index = 0; index < 8; index++) void ask(runner, [{ role: "user", content: "chat " + index }]);
  const controller = new AbortController();
  const waiting = runner.complete({ model: "m", messages: [{ role: "user", content: "title" }], tools: [] }, () => {}, controller.signal);
  const start = Date.now();
  setTimeout(() => controller.abort(), 10);
  await assert.rejects(waiting, /cancelled/);
  assert.ok(Date.now() - start < 1000, "the abort waited for the admission deadline");
});

test("idle expiry keeps conversations waiting on tool results", async (t) => {
  t.mock.timers.enable({ apis: ["Date"], now: Date.now() });
  const { runner, created } = pool();
  const chat: ChatMessage[] = [{ role: "user", content: "tool please" }];
  const first = await ask(runner, chat);
  await ask(runner, [{ role: "user", content: "plain" }]);
  t.mock.timers.tick(31 * 60 * 1000);
  await ask(runner, [{ role: "user", content: "later" }]);
  assert.equal(created[0].closed, false, "the tool-waiting session expired");
  assert.equal(created[1].closed, true, "the idle session should expire");
  await ask(runner, [...chat, { role: "assistant", content: "", tool_calls: first.toolCalls }, { role: "tool", tool_call_id: first.toolCalls[0].id, content: "found" }]);
  assert.deepEqual(created[0].actions.at(-1), { type: "tools", results: [{ id: "call-0", output: "found" }] });
});
