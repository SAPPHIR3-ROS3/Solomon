import assert from "node:assert/strict";
import { test } from "node:test";
import { createServer } from "vite";

const server = await createServer({ configFile: false, server: { middlewareMode: true, hmr: false } });
const { installCommandSelection } = await server.ssrLoadModule("/src/terminal-panel/commandSelection.ts");

test("multiline commands retain their original prompt and exclude the next prompt", () => {
  const handlers = new Map();
  const listeners = new Map();
  const buffer = { cursorY: 2, baseY: 0, viewportY: 0 };
  let selected;
  const screen = { contains: () => true, getBoundingClientRect: () => ({ top: 0, bottom: 200, height: 200 }) };
  const host = { addEventListener: (name, fn) => listeners.set(name, fn), removeEventListener() {}, querySelector: () => screen };
  const term = {
    rows: 20,
    buffer: { active: buffer, normal: buffer },
    parser: { registerOscHandler: (code, fn) => { handlers.set(code, fn); return { dispose() {} }; } },
    registerMarker: () => ({ line: buffer.cursorY, isDisposed: false, dispose() {} }),
    selectLines: (...range) => { selected = range; },
  };
  const selection = installCommandSelection(term, host);
  handlers.get(133)("A");
  selection.begin();
  buffer.cursorY = 3;
  selection.begin();
  buffer.cursorY = 4;
  handlers.get(133)("C");
  buffer.cursorY = 10;
  handlers.get(133)("A");
  const previousNode = globalThis.Node;
  globalThis.Node = class {};
  try {
    const click = (y) => {
      const event = { button: 0, detail: 1, clientX: 0, clientY: y, target: new Node() };
      listeners.get("mousedown")(event);
      listeners.get("mouseup")(event);
    };
    click(70);
    assert.deepEqual(selected, [2, 9]);
    selected = undefined;
    click(105);
    assert.equal(selected, undefined);
  } finally {
    globalThis.Node = previousNode;
    selection.dispose();
  }
});

await server.close();
