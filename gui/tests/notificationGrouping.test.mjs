import assert from "node:assert/strict";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
try {
  const { groupNotifications } = await server.ssrLoadModule("/src/notifications/store.ts");
  const entry = (id, seconds, level = "error") => ({ id, createdAt: new Date(1700000000000 + seconds * 1000).toISOString(), level, message: id });
  test("error bursts attach to the first error and preserve every message", () => {
    const groups = groupNotifications([entry("third", 4), entry("first", 0), entry("second", 2)]);
    assert.equal(groups.length, 1);
    assert.equal(groups[0].id, "first");
    assert.deepEqual(groups[0].children.map(child => child.id), ["second", "third"]);
  });
  test("warning bursts use independent bounded groups alongside errors", () => {
    const groups = groupNotifications([entry("warning-first", 0, "warning"), entry("error-first", 1), entry("warning-second", 2, "warning"), entry("error-second", 3), entry("warning-boundary", 5, "warning"), entry("warning-next", 6, "warning"), entry("warning-child", 7, "warning")]);
    assert.deepEqual(groups.map(group => group.id), ["warning-next", "error-first", "warning-first"]);
    assert.deepEqual(groups.map(group => group.children.map(child => child.id)), [["warning-child"], ["error-second"], ["warning-second", "warning-boundary"]]);
  });
  test("bursts have a bounded window and other levels remain independent", () => {
    const groups = groupNotifications([entry("first", 0), entry("system", 1, "system"), entry("warning", 2, "warning"), entry("boundary", 5), entry("next", 6), entry("child", 7)]);
    assert.deepEqual(groups.map(group => group.id), ["next", "warning", "system", "first"]);
    assert.deepEqual(groups[3].children.map(child => child.id), ["boundary"]);
    assert.deepEqual(groups[0].children.map(child => child.id), ["child"]);
  });
} finally { await server.close(); }
