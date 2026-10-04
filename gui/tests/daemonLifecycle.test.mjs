import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
const { daemonRestartTracker } = await server.ssrLoadModule("/src/daemonLifecycle.ts");
after(() => server.close());
const health = (pid, version = "v2026.1003.0", started_at = "2026-10-03T00:00:00Z") => ({ ok: true, server: { pid, version, started_at } });

test("clients reload only after a healthy new daemon is available", () => {
  const restarted = daemonRestartTracker();
  assert.equal(restarted(health(123)), false);
  assert.equal(restarted(health(123)), false);
  assert.equal(restarted(null), false);
  assert.equal(restarted({ ok: false }), false);
  assert.equal(restarted({ ok: true, server: {} }), false);
  assert.equal(restarted(health(124, "v2026.1003.1")), true);
  assert.equal(restarted(health(124, "v2026.1003.1")), false);
});

test("a reused PID still identifies a new daemon generation", () => {
  const restarted = daemonRestartTracker();
  assert.equal(restarted(health(123)), false);
  assert.equal(restarted(health(123, "v2026.1003.0", "2026-10-03T01:00:00Z")), true);
});
