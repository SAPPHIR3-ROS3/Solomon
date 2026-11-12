import assert from "node:assert/strict";
import { after, test } from "node:test";
import { existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { chromium } from "playwright-core";

const fixtureModule = `
import React, { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { MarkdownContent } from '/src/chat/MarkdownContent.tsx';
window.highlights = [];
window.runtime = { ClipboardSetText: async () => true };
function Fixture() {
  const [content, setContent] = useState('~~~js\\nconst value = 0;\\n~~~');
  window.updateContent = setContent;
  return <><section id="history"><MarkdownContent content="Historical **message**" /></section>
    <section id="live"><MarkdownContent content={content} /></section><textarea aria-label="Input" /></>;
}
createRoot(document.getElementById('root')).render(<Fixture />);
`;
const server = await createServer({
  root: fileURLToPath(new URL("..", import.meta.url)), configFile: false,
  plugins: [{
    name: "streaming-fixture",
    enforce: "pre",
    resolveId(id) {
      if (id === "/__streaming_fixture.tsx") return id;
      if (id.endsWith("/syntax/cursorSyntax")) return "virtual:highlight-counter";
    },
    load(id) {
      if (id === "/__streaming_fixture.tsx") return fixtureModule;
      if (id === "virtual:highlight-counter") return `export const cursorForeground = '#fff';
        export async function highlightCursorCode(source) {
          window.highlights.push(source);
          return source.split('\\n').map(line => [{ content: line, color: '#fff' }]);
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

test("streaming retains code blocks and copy state, and highlights only after updates settle", async () => {
  const page = await browser.newPage();
  page.setDefaultTimeout(10000);
  const errors = [];
  page.on("pageerror", error => errors.push(error.message));
  try {
    await page.route("**/__streaming_fixture", route => route.fulfill({ contentType: "text/html", body: '<div id="root"></div><script type="module" src="/__streaming_fixture.tsx"></script>' }));
    await page.goto(`${server.resolvedUrls.local[0]}__streaming_fixture`);
    await page.waitForFunction(() => window.highlights.length === 1);
    await page.getByRole("button", { name: "Copy code", exact: true }).click();
    await page.getByRole("button", { name: "Code copied", exact: true }).waitFor();
    await page.evaluate(() => {
      window.originalCode = document.querySelector("#live pre");
      window.originalHistory = document.querySelector("#history p");
      let index = 0;
      window.streamFinished = false;
      const timer = setInterval(() => {
        window.updateContent('~~~js\nconst value = ' + (++index) + ';\n~~~');
        if (index === 10) { clearInterval(timer); window.streamFinished = true; }
      }, 30);
    });
    await page.getByRole("textbox", { name: "Input" }).fill("typing during streaming");
    await page.waitForFunction(() => window.streamFinished && document.querySelector("#live code").textContent.includes("10;"));
    assert.equal(await page.evaluate(() => window.highlights.length), 1);
    assert.equal(await page.evaluate(() => document.querySelector("#live pre") === window.originalCode), true);
    assert.equal(await page.evaluate(() => document.querySelector("#history p") === window.originalHistory), true);
    await page.getByRole("button", { name: "Code copied", exact: true }).waitFor();
    assert.equal(await page.getByRole("textbox", { name: "Input" }).inputValue(), "typing during streaming");
    await page.waitForFunction(() => window.highlights.length === 2);
    assert.deepEqual(await page.evaluate(() => window.highlights), ["const value = 0;", "const value = 10;"]);
    assert.deepEqual(errors, []);
  } finally { await page.close(); }
});
