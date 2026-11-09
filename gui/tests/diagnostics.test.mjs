import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
const diagnostics = await server.ssrLoadModule("/src/diagnostics.ts");
after(() => server.close());

test("GUI errors send bounded metadata without error messages, paths, or URL credentials", async () => {
  const originalWindow = globalThis.window;
  const originalFetch = globalThis.fetch;
  const listeners = new Map();
  const requests = [];
  globalThis.window = {
    location: { protocol: "http:", hostname: "localhost" },
    setTimeout,
    clearTimeout,
    addEventListener: (type, listener) => listeners.set(type, listener),
  };
  globalThis.fetch = async (url, init) => {
    requests.push({ url, data: JSON.parse(init.body) });
    return { ok: true };
  };
  try {
    diagnostics.installDiagnostics();
    listeners.get("error")({ error: new TypeError("private error content"), filename: "https://private-host/private-directory/index.js?token=private-token", lineno: 12, colno: 34 });
    const rejected = new RangeError("private rejection content");
 rejected.stack = "RangeError: private rejection content\n at render (https://private-host/private-directory/App.tsx?token=private-token:56:78)";
 listeners.get("unhandledrejection")({ reason: rejected });
    diagnostics.reportGUIError(new DOMException("expected cancellation", "AbortError"), "promise");
    await new Promise((resolve) => setImmediate(resolve));
    assert.equal(requests.length, 2);
    assert.deepEqual(requests[0].data, { stage: "runtime", name: "TypeError", source: "index.js", line: 12, column: 34 });
    assert.deepEqual(requests[1].data, { stage: "promise", name: "RangeError", source: "App.tsx", line: 56, column: 78 });
    assert.doesNotMatch(JSON.stringify(requests), /private|expected cancellation/);
    for (let index = 0; index < 30; index += 1) diagnostics.reportGUIError(new Error("private"), "react-uncaught");
    await new Promise((resolve) => setImmediate(resolve));
    assert.equal(requests.length, 10);
  } finally {
    globalThis.window = originalWindow;
    globalThis.fetch = originalFetch;
  }
});
