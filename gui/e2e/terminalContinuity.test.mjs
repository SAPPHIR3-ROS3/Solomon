import assert from "node:assert/strict";
import { after, test } from "node:test";
import { existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { chromium } from "playwright-core";

const fixture = `
import React, { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { IntegratedShell } from '/src/terminal-panel/IntegratedShell.tsx';
window.terminals = [];
function Fixture() {
  const [visible, setVisible] = useState(false);
  const [mounted, setMounted] = useState(true);
  window.setVisible = setVisible;
  window.setMounted = setMounted;
  return <div style={{ width: 320, height: 300, display: visible ? 'block' : 'none' }}>
    {mounted && <IntegratedShell tabId="first" visible={visible}
      onTerminateReady={terminate => window.terminateTerminal = terminate}
      workingDirectory={visible ? '/workspace' : ''} onCommandSubmit={() => {}} onRunningChange={() => {}} />}
  </div>;
}
createRoot(document.getElementById('root')).render(<React.StrictMode><Fixture /></React.StrictMode>);
`;
const server = await createServer({
  root: fileURLToPath(new URL("..", import.meta.url)), configFile: false,
  plugins: [{
    name: "terminal-continuity-fixture", enforce: "pre",
    resolveId(id) {
      if (id === "/__terminal_fixture.tsx") return id;
      if (id === "@xterm/xterm") return "virtual:observed-terminal";
    },
    load(id) {
      if (id === "/__terminal_fixture.tsx") return fixture;
      if (id === "virtual:observed-terminal") return `
        import { Terminal as Xterm } from '/node_modules/@xterm/xterm/lib/xterm.mjs';
        export class Terminal extends Xterm {
          constructor(options) { super(options); window.terminals.push(this); }
        }`;
    },
  }],
  server: { host: "127.0.0.1", port: 0, hmr: false }, esbuild: { jsx: "automatic" },
});
await server.listen();
after(() => server.close());
const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/google-chrome"].find(existsSync);
assert.ok(executablePath, "Install Chromium or set SOLOMON_TEST_BROWSER");
const browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
after(() => browser.close());

test("first opening replays a saved prompt; hiding retains output; reconnect resumes; remount replays", async () => {
  const page = await browser.newPage();
  page.setDefaultTimeout(10000);
  const errors = [], connections = [];
  page.on("pageerror", error => errors.push(error.message));
  const output = [{ seq: 1, data: "RESTORED_PROMPT> " }];
  const sendOutput = (socket, item) => socket.send(JSON.stringify({ type: "solomon-output", seq: item.seq, data: Buffer.from(item.data).toString("base64") }));
  await page.routeWebSocket("**/__solomon/terminal?*", socket => {
    const url = new URL(socket.url());
    connections.push({ socket, url });
    socket.onMessage(data => {
      if (JSON.parse(String(data)).type === "close") socket.close();
    });
    socket.send(JSON.stringify({ type: "solomon-terminal", id: "saved-session" }));
    socket.send(JSON.stringify({ type: "solomon-status", running: true }));
    for (const item of output) if (item.seq > Number(url.searchParams.get("after"))) sendOutput(socket, item);
  });
  const text = () => {
    const term = window.terminals.at(-1), buffer = term?.buffer.active;
    return buffer ? Array.from({ length: buffer.length }, (_, index) => buffer.getLine(index).translateToString(true)).join("\n") : "";
  };
  try {
    await page.route("**/__terminal_fixture", route => route.fulfill({ contentType: "text/html", body: '<div id="root"></div><script type="module" src="/__terminal_fixture.tsx"></script>' }));
    await page.goto(`${server.resolvedUrls.local[0]}__terminal_fixture`);
    await page.waitForFunction(() => typeof window.setVisible === "function");
    assert.equal(connections.length, 0);
    await page.evaluate(() => {
      localStorage.setItem("solomon.terminal-session.v1.first", JSON.stringify({ id: "saved-session", seq: 1 }));
      window.setVisible(true);
    });
    await page.waitForFunction(`(${text})().includes('RESTORED_PROMPT>')`);
    assert.equal(connections.length, 1);
    assert.equal(connections[0].url.searchParams.get("session_id"), "saved-session");
    assert.equal(connections[0].url.searchParams.get("after"), null);
    assert.ok(Number(connections[0].url.searchParams.get("cols")) < 80);
    await page.evaluate(() => { window.originalTerminal = window.terminals.at(-1); window.setVisible(false); });
    await page.waitForFunction(() => document.querySelector('.terminal-panel-host').getAttribute('aria-hidden') === 'true');
    output.push({ seq: 2, data: "\r\nBACKGROUND_OUTPUT\r\n" });
    sendOutput(connections[0].socket, output[1]);
    await page.waitForFunction(`(${text})().includes('BACKGROUND_OUTPUT')`);
    await page.evaluate(() => window.setVisible(true));
    await page.waitForFunction(() => document.querySelector('.terminal-panel-host').getAttribute('aria-hidden') === 'false');
    assert.equal(await page.evaluate(() => window.originalTerminal === window.terminals.at(-1)), true);
    assert.equal(connections.length, 1);
    connections[0].socket.close();
    await new Promise((resolve, reject) => {
      const start = Date.now();
      const timer = setInterval(() => {
        if (connections.length === 2) { clearInterval(timer); resolve(); }
        else if (Date.now() - start > 10000) { clearInterval(timer); reject(new Error('Reconnect timed out')); }
      }, 25);
    });
    assert.equal(connections[1].url.searchParams.get("after"), "2");
    assert.equal((await page.evaluate(text)).split("RESTORED_PROMPT>").length, 2);
    await page.evaluate(() => window.setMounted(false));
    await page.locator('.terminal-panel-host').waitFor({ state: 'detached' });
    await page.evaluate(() => window.setMounted(true));
    await page.waitForFunction(() => window.terminals.at(-1) !== window.originalTerminal);
    await page.waitForFunction(`(${text})().includes('BACKGROUND_OUTPUT')`);
    assert.equal(connections.length, 3);
    assert.equal(connections[2].url.searchParams.get("after"), null);
    assert.ok((await page.evaluate(text)).includes("RESTORED_PROMPT>"));
    let terminated = false;
    connections[2].socket.onMessage(data => {
      if (JSON.parse(String(data)).type === "close") {
        terminated = true;
        connections[2].socket.close();
      }
    });
    await page.evaluate(() => { window.terminateTerminal(); window.setMounted(false); });
    await page.locator('.terminal-panel-host').waitFor({ state: 'detached' });
    assert.equal(terminated, true);
    assert.equal(await page.evaluate(() => localStorage.getItem('solomon.terminal-session.v1.first')), null);
    assert.deepEqual(errors, []);
  } finally { await page.close(); }
});
