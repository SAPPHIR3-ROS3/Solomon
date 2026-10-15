import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { existsSync } from "node:fs";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { build, createLogger, preview } from "vite";
import { chromium } from "playwright-core";

const root = fileURLToPath(new URL("..", import.meta.url));
const warnings = [], errors = [], requests = [];
let directory, server, browser, page, output, syntaxEntry;

before(async () => {
  directory = await mkdtemp(join(tmpdir(), "solomon-production-chunks-"));
  const logger = createLogger("warn");
  const result = await build({
    root, logLevel: "warn", customLogger: { ...logger, warn(message, options) { warnings.push(message); logger.warn(message, options); } },
    plugins: [{
      name: "production-syntax-fixture",
      resolveId(id) { if (id === "virtual:syntax-fixture") return "\0syntax-fixture"; },
      load(id) {
        if (id === "\0syntax-fixture") return `import { highlightCursorCode } from ${JSON.stringify(join(root, "src/syntax/cursorSyntax.ts"))}; window.highlightForTest = highlightCursorCode;`;
      },
    }],
    build: { outDir: directory, emptyOutDir: true, rolldownOptions: { input: { app: join(root, "index.html"), syntax: "virtual:syntax-fixture" } } },
  });
  output = result.output;
  syntaxEntry = output.find(file => file.type === "chunk" && file.isEntry && file.name === "syntax");
  assert.ok(syntaxEntry, "Production syntax test entry must exist");
  server = await preview({ root, configFile: false, build: { outDir: directory }, preview: { host: "127.0.0.1", port: 0 } });
  const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/google-chrome"].find(existsSync);
  assert.ok(executablePath, "Install Chromium or set SOLOMON_TEST_BROWSER");
  browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
  page = await browser.newPage();
  page.on("pageerror", error => errors.push(error.message));
  page.on("request", request => requests.push(request.url()));
}, { timeout: 60000 });

after(async () => {
  await browser?.close();
  if (server) await new Promise(resolve => server.httpServer.close(resolve));
  if (directory) await rm(directory, { recursive: true, force: true });
});

test("production JavaScript stays below the unchanged Vite warning threshold", () => {
  assert.equal(warnings.some(message => /chunks are larger|chunkSizeWarningLimit/.test(message)), false);
  for (const file of output.filter(file => file.type === "chunk")) {
    assert.ok(Buffer.byteLength(file.code) <= 500_000, `${file.fileName} exceeds 500 kB`);
  }
  assert.ok(output.some(file => file.type === "asset" && file.fileName.endsWith(".wasm")));
  assert.ok(output.some(file => file.type === "asset" && /syntax-.*\.json$/.test(file.fileName)));
});

test("production home and plain text do not download syntax data", async () => {
  await page.goto(server.resolvedUrls.local[0]);
  await page.locator(".welcome-stage .welcome-composer").waitFor();
  await page.addScriptTag({ url: new URL(syntaxEntry.fileName, server.resolvedUrls.local[0]).href, type: "module" });
  await page.waitForFunction(() => typeof window.highlightForTest === "function");
  const tokens = await page.evaluate(() => window.highlightForTest("plain\ntext", "text"));
  assert.deepEqual(tokens, [[{ content: "plain", offset: 0 }], [{ content: "text", offset: 6 }]]);
  assert.equal(requests.some(url => /\.(?:wasm|json)$/.test(new URL(url).pathname)), false);
  assert.deepEqual(errors, []);
});

test("production stylesheet order preserves the mobile settings controls", async () => {
  await page.setViewportSize({ width: 320, height: 640 });
  await page.evaluate(() => {
    history.pushState(null, "", "/settings");
    window.dispatchEvent(new Event("solomon-settings-location"));
  });
  await page.getByRole("button", { name: "Back to home", exact: true }).click();
  await page.locator(".welcome-stage .welcome-composer").waitFor();
  await page.setViewportSize({ width: 1280, height: 720 });
  assert.deepEqual(errors, []);
});

test("large Cursor and Shiki grammars highlight using local production assets", async () => {
  const origin = new URL(server.resolvedUrls.local[0]).origin;
  // No external request may be needed for packaged/offline highlighting.
  await page.route("**/*", route => new URL(route.request().url()).origin === origin ? route.continue() : route.abort());
  for (const language of ["javascript", "python", "cpp", "emacs-lisp", "latex"]) {
    const tokens = await page.evaluate(language => window.highlightForTest("value = 1", language), language);
    assert.equal(tokens.flat().map(token => token.content).join(""), "value = 1");
    assert.ok(tokens.flat().some(token => token.color), `${language} has no highlighted tokens`);
  }
  assert.ok(requests.some(url => /oniguruma-.*\.wasm$/.test(new URL(url).pathname)));
  assert.ok(requests.some(url => /syntax-source\.cpp-.*\.json$/.test(new URL(url).pathname)));
  assert.ok(requests.some(url => /syntax-emacs-lisp-.*\.json$/.test(new URL(url).pathname)));
  assert.deepEqual(errors, []);
});
