import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { changedLines } = await server.ssrLoadModule("/src/editor/lineChanges.ts");

test("Git markers distinguish added, modified and deleted lines", () => {
 assert.deepEqual(changedLines("a\nb\nc", "a\nx\nb\nc"), [{ line: 2, kind: "added" }]);
 assert.deepEqual(changedLines("a\nb\nc", "a\nx\nc"), [{ line: 2, kind: "modified" }]);
 assert.deepEqual(changedLines("a\nb\nc", "a\nc"), [{ line: 2, kind: "deleted" }]);
 assert.deepEqual(changedLines("a\nb", "a"), [{ line: 1, kind: "deleted" }]);
});
test("Git markers keep independent hunks on their buffer lines", () => {
 assert.deepEqual(changedLines("a\nb\nc\nd\ne", "a\nx\nc\nd\ny\ne"), [{ line: 2, kind: "modified" }, { line: 5, kind: "added" }]);
 assert.deepEqual(changedLines("a\nb\nc\nd\ne", "a\nc\nd\nx"), [{ line: 2, kind: "deleted" }, { line: 4, kind: "modified" }]);
});
test("Git markers clear on undo and handle empty and new files", () => {
 assert.deepEqual(changedLines("a\nb", "a\nb"), []);
 assert.deepEqual(changedLines("", "a\nb"), [{ line: 1, kind: "added" }, { line: 2, kind: "added" }]);
 assert.deepEqual(changedLines("a\nb", ""), [{ line: 1, kind: "deleted" }]);
 assert.deepEqual(changedLines("a\r\nb", "a\nb"), []);
});
test("large unchanged surroundings do not shift the markers", () => {
 const lines = Array.from({ length: 20000 }, (_, index) => `line ${index}`);
 const changed = [...lines]; changed[10000] = "changed";
 assert.deepEqual(changedLines(lines.join("\n"), changed.join("\n")), [{ line: 10001, kind: "modified" }]);
});
