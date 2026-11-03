import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { existsSync } from "node:fs";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import react from "@vitejs/plugin-react";
import { chromium } from "playwright-core";

const root = fileURLToPath(new URL("..", import.meta.url));
const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/chromium-browser", "/usr/bin/google-chrome"].find(existsSync);
let directory, server, browser, url;
before(async () => {
 directory = await mkdtemp(path.join(root, ".git-markers-test-"));
 await writeFile(path.join(directory, "entry.jsx"), [
  "import React from 'react';",
  "import { createRoot } from 'react-dom/client';",
  "import { CodeEditor } from '/src/editor/CodeEditor.tsx';",
  "function Harness() {",
  " const [content, setContent] = React.useState(" + JSON.stringify("a\nx\nc\ny\nd") + ");",
  " return <div style={{ height: 400 }}><CodeEditor projectID='project' file={{ id: 'file', path: 'file.txt', content, saved: content }} onChange={setContent} onCursor={() => {}} onSave={() => {}} onViewState={() => {}} /></div>;",
  "}",
  "createRoot(document.getElementById('root')).render(<React.StrictMode><Harness /></React.StrictMode>);",
 ].join("\n"));
 server = await createServer({ root, configFile: false, plugins: [react()], server: { host: "127.0.0.1", port: 0, hmr: false } });
 await server.listen();
 url = `http://127.0.0.1:${server.httpServer.address().port}`;
 browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
});
after(async () => {
 await browser?.close(); await server?.close();
 if (directory) await rm(directory, { recursive: true, force: true });
});

test("editor displays Git markers and refreshes them while typing and after commit", async () => {
 const page = await browser.newPage();
 page.setDefaultTimeout(10000);
 const errors = []; page.on("pageerror", error => errors.push(error.message));
 let content = "a\nb\nc\nd", isRepo = true;
 await page.route("**/__solomon/projects/project/file?**", route => route.fulfill({ json: { content, isRepo } }));
 const html = await server.transformIndexHtml("/marker-check", `<div id="root"></div><script type="module" src="/${path.basename(directory)}/entry.jsx"></script>`);
 await page.route("**/marker-check", route => route.fulfill({ contentType: "text/html", body: html }));
 await page.goto(`${url}/marker-check`);
 const marker = kind => page.locator(`.editor-git-marker.is-${kind}:visible`);
 await marker("modified").waitFor(); await marker("added").waitFor();
 assert.equal(await marker("modified").count(), 1);
 assert.equal(await marker("added").count(), 1);
 assert.equal(await marker("modified").evaluate(element => getComputedStyle(element).width), "3px");
 assert.match(await marker("modified").evaluate(element => getComputedStyle(element).backgroundImage), /repeating-linear-gradient/);
 const editor = page.locator(".cm-content");
 await editor.fill(content);
 await page.waitForFunction(() => [...document.querySelectorAll(".editor-git-marker")].every(element => getComputedStyle(element).visibility === "hidden" || !element.getClientRects().length));
 await editor.fill("a\nc\nd");
 await marker("deleted").waitFor();
 content = "a\nc\nd";
 await page.evaluate(() => window.dispatchEvent(new CustomEvent("solomon:git-status-changed", { detail: { projectID: "project" } })));
 await marker("deleted").waitFor({ state: "hidden" });
 await editor.fill("a\nx\nd");
 await marker("modified").waitFor();
 isRepo = false;
 await page.evaluate(() => window.dispatchEvent(new CustomEvent("solomon:git-status-changed", { detail: { projectID: "project" } })));
 await marker("modified").waitFor({ state: "hidden" });
 assert.deepEqual(errors, []);
 await page.close();
});
