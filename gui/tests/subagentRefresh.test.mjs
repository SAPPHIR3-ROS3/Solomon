import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({
  root: fileURLToPath(new URL("..", import.meta.url)), configFile: false,
  server: { middlewareMode: true, hmr: false, ws: false },

});
after(() => server.close());
const { startRefresh } = await server.ssrLoadModule("/src/chat/useSubagentRefresh.ts");

test("refresh waits for completion, aborts on cleanup, and ignores late responses", async () => {
  const previousWindow = globalThis.window;
  const timers = new Map();
  let sequence = 0;
  let resolveRequest;
  let signal;
  const fixture = {
    saved: [],
    fetch(requestSignal) {
      signal = requestSignal;
      return new Promise(resolve => { resolveRequest = resolve; });
    },
  };
  globalThis.window = {
    setTimeout(callback) { timers.set(++sequence, callback); return sequence; },
    clearTimeout(id) { timers.delete(id); },
  };
  try {
    const cleanup = startRefresh(async (signal) => {
      const chat = await fixture.fetch(signal);
      if (!signal.aborted) fixture.saved.push(chat);
    });
    const runTimer = () => {
      const [id, callback] = timers.entries().next().value;
      timers.delete(id);
      return callback();
    };
    const first = runTimer();
    assert.equal(timers.size, 0);
    resolveRequest({ id: "chat", messages: [] });
    await first;
    assert.equal(fixture.saved.length, 1);
    assert.equal(timers.size, 1);
    const second = runTimer();
    cleanup();
    assert.equal(signal.aborted, true);
    resolveRequest({ id: "chat", messages: [] });
    await second;
    assert.equal(fixture.saved.length, 1);
    assert.equal(timers.size, 0);
  } finally {
    globalThis.window = previousWindow;
  }
});
