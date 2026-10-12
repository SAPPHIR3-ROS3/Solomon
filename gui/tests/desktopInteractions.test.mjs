import assert from "node:assert/strict";
import { test, after } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
const { watchExternalLinks } = await server.ssrLoadModule("/src/externalLinks.ts");
const { copyText } = await server.ssrLoadModule("/src/chat/chatClipboard.ts");
const originalNavigator = Object.getOwnPropertyDescriptor(globalThis, "navigator");
after(async () => {
  delete globalThis.window;
  delete globalThis.document;
  delete globalThis.Element;
  if (originalNavigator) Object.defineProperty(globalThis, "navigator", originalNavigator);
  else delete globalThis.navigator;
  await server.close();
});

test("desktop links open in the system browser and leave local or unsafe URLs alone", () => {
  const opened = [];
  const listeners = new Map();
  globalThis.window = { runtime: { BrowserOpenURL: url => opened.push(url) }, location: { href: "http://wails.localhost/" } };
  globalThis.document = { addEventListener: (name, fn) => listeners.set(name, fn), removeEventListener: name => listeners.delete(name) };
  globalThis.Element = class { constructor(href) { this.href = href; } closest() { return this; } getAttribute() { return this.href; } };
  const cleanup = watchExternalLinks();
  for (const [href, expected] of [["https://example.com/doc", true], ["mailto:hello@example.com", true], ["//example.com/doc", true], ["https://", false], ["/settings", false], ["#section", false], ["javascript:alert(1)", false], ["file:///tmp/file", false]]) {
    const event = { button: 0, defaultPrevented: false, target: new Element(href), preventDefault() { this.defaultPrevented = true; } };
    listeners.get("click")(event);
    assert.equal(event.defaultPrevented, expected, href);
  }
  assert.deepEqual(opened, ["https://example.com/doc", "mailto:hello@example.com", "https://example.com/doc"]);
  const middle = { button: 1, target: new Element("https://example.com/"), preventDefault() {} };
  listeners.get("auxclick")(middle);
  assert.equal(opened.at(-1), "https://example.com/");
  delete window.runtime;
  const browser = { button: 0, target: new Element("https://example.com/"), preventDefault() { assert.fail("web links must retain browser behavior"); } };
  listeners.get("click")(browser);
  cleanup();
  assert.equal(listeners.size, 0);
});

test("chat copy uses native clipboard and falls back to browser clipboard on failure", async () => {
  const copied = [];
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: { clipboard: { writeText: async text => copied.push(["web", text]) } } });
  globalThis.window = { runtime: { ClipboardSetText: async text => { copied.push(["native", text]); return true; } } };
  await copyText("chat text\nsecond line");
  assert.deepEqual(copied, [["native", "chat text\nsecond line"]]);
  window.runtime.ClipboardSetText = async () => { throw new Error("unavailable"); };
  await copyText("fallback");
  assert.deepEqual(copied.at(-1), ["web", "fallback"]);
  window.runtime.ClipboardSetText = async () => false;
  await copyText("retry");
  assert.deepEqual(copied.at(-1), ["web", "retry"]);
});
