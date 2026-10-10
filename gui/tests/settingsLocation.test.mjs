import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const root = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({ root, server: { middlewareMode: true, hmr: false } });
const { hideSettings, isSettingsPath, showSettings, subscribeSettingsLocation } = await server.ssrLoadModule("/src/settings/SettingsPage.tsx");
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

test("only /settings is the settings page", () => {
  installWindow("/");
  assert.equal(isSettingsPath("/"), false);
  assert.equal(isSettingsPath("/settings-extra"), false);
  assert.equal(isSettingsPath("/settings/chat"), false);
  assert.equal(isSettingsPath("/settings/models"), false);
  assert.equal(isSettingsPath("/settings/docs"), false);
  assert.equal(isSettingsPath("/settings"), true);
  assert.equal(isSettingsPath("/settings/"), true);
});

test("opening and leaving settings stays on /settings", () => {
  const pushes = installWindow("/");
  const seen = [];
  const unsubscribe = subscribeSettingsLocation(() => seen.push(window.location.pathname));
  showSettings();
  showSettings();
  hideSettings();
  hideSettings();
  assert.deepEqual(pushes, ["/settings", "/"]);
  assert.deepEqual(seen, ["/settings", "/settings", "/"]);
  assert.equal(isSettingsPath(), false);
  unsubscribe();
  showSettings();
  assert.deepEqual(seen, ["/settings", "/settings", "/"]);
  assert.equal(window.location.pathname, "/settings");
});

test("the browser back button closes settings without a section address", () => {
  installWindow("/settings");
  const seen = [];
  subscribeSettingsLocation(() => seen.push(window.location.pathname));
  window.location.pathname = "/";
  window.dispatchEvent(new Event("popstate"));
  assert.deepEqual(seen, ["/"]);
  assert.equal(isSettingsPath(), false);
});
