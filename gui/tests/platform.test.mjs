import assert from "node:assert/strict";
import { test, after } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const root = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({ root, configFile: false, server: { middlewareMode: true, hmr: false } });
const { serverEndpoint } = await server.ssrLoadModule("/src/platform.ts");
const { terminalSocketUrl } = await server.ssrLoadModule("/src/terminal-panel/terminalSocket.ts");
after(async () => { delete globalThis.window; await server.close(); });

function desktop(URL) {
  globalThis.window = {
    location: { hostname: "wails.localhost", href: "http://wails.localhost/" },
    setTimeout,
    go: { main: { ServerBridge: { URL } } },
  };
}

test("a cold desktop load uses the daemon before runtime.Environment arrives", async () => {
  desktop(async () => "http://localhost:64123/");
  assert.equal(await serverEndpoint("/__solomon/projects"), "http://localhost:64123/__solomon/projects");
  assert.equal(await terminalSocketUrl("/home/example"), "ws://localhost:64123/__solomon/terminal?path=%2Fhome%2Fexample");
});

test("Linux and macOS custom-scheme clients discover the daemon on a cold load", async () => {
  desktop(async () => "http://localhost:64000");
  window.location = { protocol: "wails:", hostname: "wails", href: "wails://wails/" };
  assert.equal(await serverEndpoint("/__solomon/projects"), "http://localhost:64000/__solomon/projects");
});

test("desktop requests wait for a late injected bridge", async () => {
  desktop(async () => "http://localhost:64000");
  const bridge = window.go;
  delete window.go;
  setTimeout(() => { window.go = bridge; }, 30);
  assert.equal(await serverEndpoint("/__solomon/models"), "http://localhost:64000/__solomon/models");
});

test("a missing daemon URL reports an error instead of contacting port 80", async () => {
  desktop(async () => "");
  await assert.rejects(serverEndpoint("/__solomon/projects"), /daemon is unavailable/);
});

test("web clients keep relative API URLs", async () => {
  globalThis.window = { location: { hostname: "localhost", href: "http://localhost:64000/" } };
  assert.equal(await serverEndpoint("/__solomon/projects"), "/__solomon/projects");
});
