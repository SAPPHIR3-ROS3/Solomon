import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const root = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({ root, server: { middlewareMode: true, hmr: false } });
const { hideSettings, isSettingsPath, settingsSection, showSettings, subscribeSettingsLocation } = await server.ssrLoadModule("/src/settings/SettingsPage.tsx");
after(() => server.close());

function installWindow(pathname) {
  const listeners = new Map();
  const pushes = [];
  globalThis.window = {
    location: { pathname },
    history: {
      pushState(_data, _title, url) {
        pushes.push(url);
        window.location.pathname = url;
      },
    },
    addEventListener(type, listener) {
      const current = listeners.get(type) ?? new Set();
      current.add(listener);
      listeners.set(type, current);
    },
    removeEventListener(type, listener) {
      listeners.get(type)?.delete(listener);
    },
    dispatchEvent(event) {
      for (const listener of listeners.get(event.type) ?? []) listener(event);
      return true;
    },
  };
  return pushes;
}

test("settings routes recognize the page and its sections", () => {
  installWindow("/");
  assert.equal(isSettingsPath("/"), false);
  assert.equal(isSettingsPath("/settings-extra"), false);
  assert.equal(isSettingsPath("/settings"), true);
  assert.equal(isSettingsPath("/settings/"), true);
  assert.equal(isSettingsPath("/settings/chat"), true);
  assert.equal(isSettingsPath("/settings/models/"), true);
  assert.equal(isSettingsPath("/settings/docs"), true);
  assert.equal(isSettingsPath("/settings/other"), true);
  assert.equal(settingsSection("/settings"), "");
  assert.equal(settingsSection("/settings/"), "");
  assert.equal(settingsSection("/settings/chat/"), "chat");
  assert.equal(settingsSection("/settings/models"), "models");
  assert.equal(settingsSection("/settings/docs"), "docs");
  assert.equal(settingsSection("/settings/other"), "");
});

test("opening and leaving settings updates the address and subscribers", () => {
  const pushes = installWindow("/");
  const seen = [];
  const unsubscribe = subscribeSettingsLocation(() => seen.push(window.location.pathname));
  showSettings();
  showSettings("models");
  showSettings("models");
  showSettings("docs");
  hideSettings();
  hideSettings();
  assert.deepEqual(pushes, ["/settings", "/settings/models", "/settings/docs", "/"]);
  assert.deepEqual(seen, ["/settings", "/settings/models", "/settings/models", "/settings/docs", "/"]);
  assert.equal(settingsSection(), "");
  assert.equal(isSettingsPath(), false);
  unsubscribe();
  showSettings("chat");
  assert.deepEqual(seen, ["/settings", "/settings/models", "/settings/models", "/settings/docs", "/"]);
  assert.equal(window.location.pathname, "/settings/chat");
});

test("the browser back button keeps subscribers on the restored settings path", () => {
  installWindow("/settings/docs");
  const seen = [];
  subscribeSettingsLocation(() => seen.push([window.location.pathname, settingsSection()]));
  window.location.pathname = "/settings";
  window.dispatchEvent(new Event("popstate"));
  window.location.pathname = "/";
  window.dispatchEvent(new Event("popstate"));
  assert.deepEqual(seen, [["/settings", ""], ["/", ""]]);
});
