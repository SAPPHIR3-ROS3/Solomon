import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { existsSync } from "node:fs";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import react from "@vitejs/plugin-react";
import { chromium } from "playwright-core";

const root = fileURLToPath(new URL("..", import.meta.url));
const source = "root\n  parent\n    child\n\n    sibling\n  other\noutside";
const fixture = `import React from 'react';
import {createRoot} from 'react-dom/client';
import {CodeEditor} from '/src/editor/CodeEditor.tsx';
import {EditorView} from '@codemirror/view';
import {undo} from '@codemirror/commands';
import '/src/styles.css';
import '/src/typography.css';
import '/src/editor/editor.css';
window.EditorView=EditorView;window.undo=undo;
function Harness(){const[content,setContent]=React.useState(${JSON.stringify(source)});return <div className='editor-page' style={{position:'relative',height:600,display:'block'}}><CodeEditor projectID='guides' file={{id:'guides',path:window.guidePath??'guides.txt',content,saved:content}} onChange={setContent} onCursor={()=>{}} onSave={()=>{}} onViewState={()=>{}}/></div>}
createRoot(document.getElementById('root')).render(<Harness/>);`;
let server, browser, cacheDir;
before(async () => {
  cacheDir = await mkdtemp(path.join(tmpdir(), "solomon-guides-"));
  server = await createServer({ root, cacheDir, configFile: false, plugins: [react(), {
    name: "indentation-guide-fixture",
    resolveId: id => id === "/__guides.jsx" ? id : null,
    load: id => id === "/__guides.jsx" ? fixture : null,
  }], server: { host: "127.0.0.1", port: 0, hmr: false } });
  await server.listen();
  const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/chromium-browser", "/usr/bin/google-chrome"].find(existsSync);
  browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
});
after(async () => { await browser?.close(); await server?.close(); if (cacheDir) await rm(cacheDir, { recursive: true, force: true }); });

async function openEditor(t, mac = false, filePath = "guides.txt") {
  const page = await browser.newPage();
  t.after(() => page.close());
  await page.addInitScript(filePath => { window.guidePath = filePath; }, filePath);
  if (mac) await page.addInitScript(() => Object.defineProperty(navigator, "platform", { get: () => "MacIntel" }));
  await page.route("**/__solomon/**", route => route.fulfill({ json: { content: source, isRepo: false } }));
  await page.route("**/guide-check", route => route.fulfill({ contentType: "text/html", body: '<div id="root"></div><script type="module" src="/__guides.jsx"></script>' }));
  await page.goto(`${server.resolvedUrls.local[0]}guide-check`);
  await page.locator(".cm-editor").waitFor();
  await page.evaluate(() => { window.view = window.EditorView.findFromDOM(document.querySelector(".cm-editor")); });
  return page;
}

async function clickGuide(page, number, column, modifiers = []) {
  const point = await page.evaluate(({ number, column }) => {
    const line = view.state.doc.line(number);
    const rect = view.coordsAtPos(line.from);
    return { x: rect.left + column * view.defaultCharacterWidth, y: (rect.top + rect.bottom) / 2 };
  }, { number, column });
  for (const modifier of modifiers) await page.keyboard.down(modifier);
  await page.mouse.click(point.x, point.y);
  for (const modifier of modifiers) await page.keyboard.up(modifier);
}

async function cursorColumns(page) {
  return page.evaluate(() => view.state.selection.ranges.map(range => {
    const line = view.state.doc.lineAt(range.head);
    return { line: line.number, column: range.head - line.from };
  }));
}

test("guides span nested blocks and blank lines and stop at dedentation", async t => {
  const page = await openEditor(t);
  assert.deepEqual(await page.locator(".cm-line").evaluateAll(lines => lines.map(line => line.getAttribute("data-indent-guides"))), [null, "0", "0,2", "0,2", "0,2", "0", null]);
  await clickGuide(page, 2, 0);
  assert.deepEqual(await cursorColumns(page), [{ line: 2, column: 0 }]);
  await clickGuide(page, 3, 2);
  assert.deepEqual(await cursorColumns(page), [{ line: 3, column: 2 }]);
  assert.equal(await page.evaluate(() => view.state.doc.toString()), source);
  await page.evaluate(() => view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: "root\n  child\noutside" } }));
  assert.deepEqual(await page.locator(".cm-line").evaluateAll(lines => lines.map(line => line.getAttribute("data-indent-guides"))), [null, "0", null]);
});

test("clicking a blank guide inserts undoable padding and supports Ctrl-click cursors", async t => {
  const page = await openEditor(t);
  await clickGuide(page, 4, 2);
  assert.deepEqual(await cursorColumns(page), [{ line: 4, column: 2 }]);
  assert.equal(await page.evaluate(() => view.state.doc.line(4).text), "  ");
  await page.evaluate(() => undo(view));
  assert.equal(await page.evaluate(() => view.state.doc.line(4).text), "");
  await clickGuide(page, 3, 2);
  await clickGuide(page, 4, 2, ["Control"]);
  assert.deepEqual(await cursorColumns(page), [{ line: 3, column: 2 }, { line: 4, column: 2 }]);
  await page.keyboard.insertText("x");
  assert.deepEqual(await page.evaluate(() => [view.state.doc.line(3).text, view.state.doc.line(4).text]), ["  x  child", "  x"]);
});

test("a guide inside a tab remains clickable at the exact visual column", async t => {
  const page = await openEditor(t);
  await page.evaluate(() => view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: "  parent\n\tchild" } }));
  await clickGuide(page, 2, 2);
  assert.deepEqual(await cursorColumns(page), [{ line: 2, column: 2 }]);
  assert.equal(await page.evaluate(() => view.state.doc.line(2).text), "    child");
  await page.evaluate(() => undo(view));
  assert.equal(await page.evaluate(() => view.state.doc.line(2).text), "\tchild");
});

test("Cmd-click adds a cursor on an empty guide on macOS", async t => {
  const page = await openEditor(t, true);
  await clickGuide(page, 3, 2);
  await clickGuide(page, 4, 2, ["Meta"]);
  assert.deepEqual(await cursorColumns(page), [{ line: 3, column: 2 }, { line: 4, column: 2 }]);
});

test("Python comments retain nested guides and clicks preserve the comment marker", async t => {
  const page = await openEditor(t, false, "guides.py");
  const content = "# parent\n#   nested\n#     child\n#\n#     sibling\n# outside";
  await page.evaluate(content => view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: content } }), content);
  assert.deepEqual(await page.locator(".cm-line").evaluateAll(lines => lines.map(line => line.getAttribute("data-indent-guides"))), [null, "2", "2,4", "2,4", "2,4", null]);
  await clickGuide(page, 3, 4);
  assert.deepEqual(await cursorColumns(page), [{ line: 3, column: 4 }]);
  assert.equal(await page.evaluate(() => view.state.doc.toString()), content);
  await clickGuide(page, 4, 4, ["Control"]);
  assert.deepEqual(await cursorColumns(page), [{ line: 3, column: 4 }, { line: 4, column: 4 }]);
  assert.equal(await page.evaluate(() => view.state.doc.line(4).text), "#   ");
  await page.evaluate(() => undo(view));
  assert.equal(await page.evaluate(() => view.state.doc.line(4).text), "#");
});

test("slash comments retain guides after their marker", async t => {
  const page = await openEditor(t, false, "guides.ts");
  await page.evaluate(() => view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: "// parent\n//   child\n//\n//   sibling\n// outside" } }));
  assert.deepEqual(await page.locator(".cm-line").evaluateAll(lines => lines.map(line => line.getAttribute("data-indent-guides"))), [null, "3", "3", "3", null]);
  await clickGuide(page, 3, 3);
  assert.deepEqual(await cursorColumns(page), [{ line: 3, column: 3 }]);
  assert.equal(await page.evaluate(() => view.state.doc.line(3).text), "// ");
});
