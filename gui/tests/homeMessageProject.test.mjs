import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
const originalFetch = globalThis.fetch;
const originalWindow = globalThis.window;
globalThis.window = { location: { hostname: "localhost", href: "http://localhost/" }, addEventListener() {}, dispatchEvent() {} };
const { resolveMessageProject } = await server.ssrLoadModule("/src/projects/projects.ts");
after(async () => {
  globalThis.fetch = originalFetch;
  globalThis.window = originalWindow;
  await server.close();
});

const home = { id: "a".repeat(64), name: "Home", path: "C:/Users/example", chats: [], chatCount: 0 };

test("sending from an unregistered Home resolves the user directory through the daemon", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => {
    requests.push({ url, method: options.method, body: JSON.parse(options.body) });
    return Response.json({ project: home }, { status: 201 });
  };
  assert.deepEqual(await resolveMessageProject(null), home);
  assert.deepEqual(requests, [{ url: "/__solomon/projects", method: "POST", body: { path: "" } }]);
});

test("sending from a selected workspace keeps its project without registering Home", async () => {
  globalThis.fetch = async () => { throw new Error("Unexpected request"); };
  assert.equal(await resolveMessageProject(home), home);
});

test("Home registration errors reach the composer so the draft can be retained", async () => {
  globalThis.fetch = async () => Response.json({ error: "Home directory is unavailable" }, { status: 500 });
  await assert.rejects(resolveMessageProject(null), /Home directory is unavailable/);
});
