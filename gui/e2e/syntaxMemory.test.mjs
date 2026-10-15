import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { existsSync } from "node:fs";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { chromium } from "playwright-core";

let server, browser, page, cache;
const requests = [];
const errors = [];
const isEngineRequest = url => /(?:oniguruma|wasm-inlined|shikijs_core)/.test(url);

before(async () => {
  cache = await mkdtemp(join(tmpdir(), "solomon-syntax-memory-"));
  server = await createServer({
    root: fileURLToPath(new URL("..", import.meta.url)), cacheDir: cache,
    server: { host: "127.0.0.1", port: 0 },
  });
  await server.listen();
  const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/google-chrome"].find(existsSync);
  assert.ok(executablePath, "Install Chromium or set SOLOMON_TEST_BROWSER");
  browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
  page = await browser.newPage();
  page.on("request", request => requests.push(request.url()));
  page.on("pageerror", error => errors.push(error.message));
  await page.route("**/__syntax_memory", route => route.fulfill({ contentType: "text/html", body: "<!doctype html><title>Syntax memory test</title>" }));
  await page.goto(`${server.resolvedUrls.local[0]}__syntax_memory`);
}, { timeout: 30000 });

after(async () => {
  await browser?.close();
  await server?.close();
  if (cache) await rm(cache, { recursive: true, force: true });
});

test("plain code preserves text and offsets without loading a language catalog or engine", async () => {
  const results = await page.evaluate(async () => {
    const { highlightCursorCode } = await import("/src/syntax/cursorSyntax.ts");
    return Promise.all(["", "text", "txt", "plain", "plaintext"].map(language => highlightCursorCode("π\r\n\nlast\n", language)));
  });
  for (const result of results) assert.deepEqual(result, [
    [{ content: "π", offset: 0 }], [{ content: "", offset: 3 }],
    [{ content: "last", offset: 4 }], [{ content: "", offset: 9 }],
  ]);
  assert.equal(requests.some(url => /bundledLanguages/.test(url) || isEngineRequest(url)), false);
  assert.deepEqual(errors, []);
});

test("unknown editor files remain plain without initializing the engine", async () => {
  const result = await page.evaluate(async () => {
    const { cursorTokenizer, highlightCursorCode } = await import("/src/syntax/cursorSyntax.ts");
    const tokenizer = await cursorTokenizer("notes.unknown-solomon-format");
    return { hasEngine: Boolean(tokenizer.highlighter), tokens: await highlightCursorCode("unchanged", "notes.unknown-solomon-format") };
  });
  assert.deepEqual(result, { hasEngine: false, tokens: [[{ content: "unchanged", offset: 0 }]] });
  assert.equal(requests.some(isEngineRequest), false);
  assert.deepEqual(errors, []);
});

test("real languages still initialize and highlight after plain code", async () => {
  const results = await page.evaluate(async () => {
    const { highlightCursorCode } = await import("/src/syntax/cursorSyntax.ts");
    return Promise.all(["javascript", "python", "go", "rust", "vue"].map(language => highlightCursorCode("value = 1", language)));
  });
  for (const tokens of results) {
    assert.equal(tokens.flat().map(token => token.content).join(""), "value = 1");
    assert.ok(tokens.flat().some(token => token.color));
  }
  assert.equal(requests.some(isEngineRequest), true);
  assert.deepEqual(errors, []);
});

test("the embedded manual loads when Docs is opened, rather than on the home screen", async () => {
  await page.goto(server.resolvedUrls.local[0]);
  await page.locator(".welcome-stage .welcome-composer").waitFor();
  assert.equal(requests.some(url => /DocsViewer|\/docs\/.*\.md/.test(url)), false);
  await page.evaluate(async () => {
    const { showSettings } = await import("/src/settings/SettingsPage.tsx");
    showSettings();
  });
  await page.getByRole("button", { name: "Docs", exact: true }).click();
  await page.locator(".settings-docs").waitFor();
  assert.ok(requests.some(url => /DocsViewer/.test(url)));
  assert.ok(requests.some(url => /\/docs\/.*\.md/.test(url)));
  assert.deepEqual(errors, []);
});
