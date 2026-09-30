import assert from "node:assert/strict";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
const { moveTerminalTabs, resizedPaneWeights, terminalGridColumns } = await server.ssrLoadModule("/src/terminal-panel/terminalLayout.ts");
const tab = (id) => ({ id, title: "Terminal", hasRunCommand: true, isRunning: true });
const pane = (id, tabs) => ({ id, tabs, activeTabId: tabs[0].id, weight: 1 });

test("reorders in both directions without replacing running terminal objects", () => {
  const tabs = [tab("a"), tab("b"), tab("c")];
  const original = [pane("left", tabs)];
  const forward = moveTerminalTabs(original, "a", "left", "c");
  assert.deepEqual(forward[0].tabs.map((t) => t.id), ["b", "c", "a"]);
  assert.equal(forward[0].tabs[2], tabs[0]);
  const backward = moveTerminalTabs(forward, "a", "left", "b");
  assert.deepEqual(backward[0].tabs.map((t) => t.id), ["a", "b", "c"]);
  assert.deepEqual(original[0].tabs.map((t) => t.id), ["a", "b", "c"]);
});

test("moves between panes, updates selection, and removes only empty panes", () => {
  const a = tab("a"), b = tab("b"), c = tab("c");
  const original = [pane("left", [a, b]), pane("right", [c])];
  const moved = moveTerminalTabs(original, "a", "right", "c");
  assert.equal(moved[0].activeTabId, "b");
  assert.equal(moved[1].activeTabId, "a");
  assert.equal(moved[1].tabs[0], a);
  const emptied = moveTerminalTabs(moved, "b", "right");
  assert.equal(emptied.length, 1);
  assert.deepEqual(emptied[0].tabs.map((t) => t.id), ["a", "c", "b"]);
  assert.equal(emptied[0].tabs[2], b);
  assert.equal(moveTerminalTabs(original, "missing", "right"), original);
});

test("resizing preserves combined width and clamps both panes to 120 pixels", () => {
  assert.deepEqual(resizedPaneWeights(1, 1, 3, 900, 150), [1.5, 0.5]);
  const [left, right] = resizedPaneWeights(1, 1, 3, 900, 10000);
  assert.equal(left + right, 2);
  assert.ok(Math.abs(right * 900 / 3 - 120) < 1e-9);
  assert.deepEqual(resizedPaneWeights(1, 1, 8, 400, 10000), [1, 1]);
});

test("remaining columns fill the panel after closing a resized split", () => {
  const remaining = { ...pane("left", [tab("a")]), weight: 0.775 };
  assert.equal(terminalGridColumns([remaining]), "minmax(0, 100fr)");
  const other = { ...pane("right", [tab("b")]), weight: 0.225 };
  assert.equal(terminalGridColumns([remaining, other]), "minmax(0, 77.5fr) minmax(0, 22.5fr)");
  assert.equal(terminalGridColumns([other]), "minmax(0, 100fr)");
});

await server.close();
