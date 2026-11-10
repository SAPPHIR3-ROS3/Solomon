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
const fixture = `import React from 'react';import {createRoot} from 'react-dom/client';
import {SettingsPage} from '/src/settings/SettingsPage.tsx';import {CodeEditor} from '/src/editor/CodeEditor.tsx';import {EditorView} from '@codemirror/view';
import '/src/styles.css';import '/src/theme/themes.css';import '/src/typography.css';import '/src/editor/editor.css';window.EditorView=EditorView;
function Harness(){const[content,setContent]=React.useState('value = 1\\npri');const[editor,setEditor]=React.useState(true);const[statusHost,setStatusHost]=React.useState(null);window.closeEditor=()=>setEditor(false);return <><style>{'.editor-page > .editor-code-container{flex:1}'}</style><SettingsPage onHome={()=>{}}/>{editor?<div className='editor-page' style={{position:'fixed',bottom:0,right:0,width:450,height:300,display:'flex',flexDirection:'column',zIndex:50}}><CodeEditor statusBar={statusHost} projectID='project' file={{id:'file',path:'example.py',content,saved:'value = 1\\npri'}} onChange={setContent} onSave={()=>{}} onCursor={()=>{}} onViewState={()=>{}}/><footer className='editor-status'><span>UTF-8</span><div className='editor-language-slot' ref={setStatusHost}/></footer></div>:null}</>};createRoot(document.getElementById('root')).render(<Harness/>);`;
let server, browser, cacheDir;
before(async () => {
  cacheDir = await mkdtemp(path.join(tmpdir(), "solomon-language-tests-"));
  server = await createServer({ root, cacheDir, configFile: false, plugins: [react(), {
    name: "language-preferences-fixture", resolveId: id => id === "/__languages.jsx" ? id : null, load: id => id === "/__languages.jsx" ? fixture : null,
  }], server: { host: "127.0.0.1", port: 0, hmr: false } });
  await server.listen();
  const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/chromium-browser", "/usr/bin/google-chrome"].find(existsSync);
  browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
});
after(async () => { await browser?.close(); await server?.close(); if (cacheDir) await rm(cacheDir, { recursive: true, force: true }); });

async function openPage(t, available = false) {
  const page = await browser.newPage({ viewport: { width: 1400, height: 1000 } });
  page.setDefaultTimeout(10000);
  page.on("pageerror", error => console.error("Language fixture page error:", error.message));
  t.after(() => page.close());
  const rows = new Map();
  await page.addInitScript(() => {
    window.languageMessages = [];
    window.languageSockets = [];
    const NativeSocket = window.WebSocket;
    class MockSocket extends EventTarget {
      static CONNECTING = 0; static OPEN = 1; static CLOSING = 2; static CLOSED = 3;
      readyState = 0;
      constructor(url, protocols) { super(); if (!String(url).includes("/__solomon/language-server")) return new NativeSocket(url, protocols); this.url = url; window.languageSockets.push(this); queueMicrotask(() => { this.readyState = 1; this.dispatchEvent(new Event("open")); }); }
      emit(data) { queueMicrotask(() => { if (this.readyState === 1) this.dispatchEvent(new MessageEvent("message", { data: JSON.stringify(data) })); }); }
      send(raw) {
        const message = JSON.parse(raw); window.languageMessages.push(message);
        if (message.method === "shutdown") this.emit({ id: message.id, result: null });
        if (message.method === "initialize") this.emit({ id: message.id, result: { capabilities: { positionEncoding: "utf-16", textDocumentSync: { change: 2, save: true }, completionProvider: {}, hoverProvider: true } } });
        if (message.method === "textDocument/didOpen") {
          this.emit({ id: 90, method: "workspace/configuration", params: { items: [{ section: "python" }] } });
          this.emit({ method: "textDocument/publishDiagnostics", params: { uri: message.params.textDocument.uri, version: 1, diagnostics: [{ range: { start: { line: 0, character: 0 }, end: { line: 0, character: 5 } }, message: "Fixture diagnostic", severity: 1 }] } });
        }
        if (message.method === "textDocument/completion") this.emit({ id: message.id, result: [{ label: "print", insertText: "print" }] });
        if (message.method === "textDocument/hover") this.emit({ id: message.id, result: { contents: { kind: "plaintext", value: "Fixture type information" } } });
      }
      close() { if (this.readyState === 3) return; this.readyState = 3; this.dispatchEvent(new Event("close")); }
    }
    window.WebSocket = MockSocket;
  });
  await page.route("**/__solomon/**", async route => {
    const request = route.request(), url = new URL(request.url());
    if (url.pathname === "/__solomon/programming-languages") {
      if (request.method() !== "GET") {
        const patch = request.postDataJSON();
        const previous = rows.get(patch.id);
        if (!previous || request.method() === "PATCH") rows.set(patch.id, { ...patch, firstUsed: previous?.firstUsed ?? "2026-11-10", lastUsed: "2026-11-10", settings: patch.settings ?? {}, server: { available: patch.mode !== "fallback" && available, command: patch.mode === "custom" ? patch.command : ["fixture-lsp", "--stdio"], source: "detected", reason: "No compatible installed language server detected" }, servers: available ? [{ available: true, command: ["fixture-lsp", "--stdio"], source: "detected" }, { available: true, command: ["second-lsp", "--stdio"], source: "detected" }] : [] });
      }
      await route.fulfill({ json: [...rows.values()] }); return;
    }
    if (url.pathname === "/__solomon/language-document") { await route.fulfill({ json: { rootUri: "file:///workspace", uri: "file:///workspace/example.py" } }); return; }
    if (url.pathname === "/__solomon/gui-settings") { await route.fulfill({ json: { general: { userName: "Fixture", responseLanguage: "English" }, chat: {}, advanced: {} } }); return; }
    await route.fulfill({ json: { content: "value = 1\npri", isRepo: false, providers: [] } });
  });
  await page.route("**/language-check", route => route.fulfill({ contentType: "text/html", body: '<div id="root" class="app-shell"></div><script type="module" src="/__languages.jsx"></script>' }));
  await page.goto(`${server.resolvedUrls.local[0]}language-check`);
  await page.locator(".cm-editor").waitFor();
  return { page, rows };
}

test("Programming Languages appears after Network and persists custom and fallback choices", async t => {
  const { page, rows } = await openPage(t);
  await page.waitForFunction(() => document.querySelector('.editor-language-status')?.getAttribute("title")?.includes("Python"));
  const labels = await page.locator('.settings-navigation button').allTextContents();
  assert.equal(labels.indexOf("Programming Languages"), labels.indexOf("Network") + 1);
  const navigation = await page.getByRole("button", { name: "Programming Languages", exact: true }).evaluate(button => {
    const label = button.querySelector("span"), range = document.createRange();
    range.selectNodeContents(label);
    return { lines: range.getClientRects().length, right: label.getBoundingClientRect().right, panelRight: button.closest("aside").getBoundingClientRect().right };
  });
  assert.equal(navigation.lines, 1);
  assert.ok(navigation.right < navigation.panelRight);
  await page.getByRole("button", { name: "Programming Languages", exact: true }).click();
  await page.getByRole("button", { name: /Python.*Internal fallback/ }).click();
  const assistance = page.getByRole("combobox", { name: "Language assistance" });
  assert.equal(await assistance.evaluate(element => element.classList.contains("shared-select-trigger")), true);
  assert.equal(await page.locator(".programming-language-preferences select").count(), 0);
  await assistance.click();
  await page.getByRole("option", { name: "Custom language server", exact: true }).click();
  await page.getByRole("textbox", { name: "Python server command" }).fill('["/custom/server", "--stdio"]');
  await page.getByRole("textbox", { name: "Python server settings" }).fill('{"python":{"analysis":{"typeCheckingMode":"strict"}}}');
  await page.getByLabel("Diagnostics", { exact: true }).uncheck();
  await page.getByRole("button", { name: "Save language settings", exact: true }).click();
  await page.getByRole("button", { name: "Save language settings", exact: true }).waitFor();
  await page.waitForFunction(() => document.querySelector('.editor-language-status')?.getAttribute("title")?.includes("Internal fallback"));
  assert.deepEqual(rows.get("python").command, ["/custom/server", "--stdio"]);
  assert.equal(rows.get("python").diagnostics, false);
  assert.equal(rows.get("python").settings.python.analysis.typeCheckingMode, "strict");
  await assistance.click();
  await page.getByRole("option", { name: "Internal fallback only", exact: true }).click();
  await page.getByRole("button", { name: "Save language settings", exact: true }).click();
  await page.getByRole("button", { name: "Save language settings", exact: true }).waitFor();
  assert.equal(rows.get("python").mode, "fallback");
  assert.equal(await page.evaluate(() => languageSockets.length), 0);
});

test("installed LSP initializes, synchronizes, supplies diagnostics/completion/hover and closes with editor", async t => {
  const { page } = await openPage(t, true);
  await page.waitForFunction(() => document.querySelector('.editor-language-status')?.getAttribute("title")?.includes("LSP: fixture-lsp"));
  await page.locator('.language-error[title="Fixture diagnostic"]').waitFor();
  await page.waitForFunction(() => languageMessages.some(message => message.id === 90 && !message.method));
  assert.deepEqual(await page.evaluate(() => languageMessages.find(message => message.id === 90 && !message.method).result), [null]);
  await page.locator('.cm-content').click();
  await page.keyboard.press('Control+End');
  await page.keyboard.insertText('n');
  await page.keyboard.press('Control+Space');
  await page.getByRole('option', { name: 'print', exact: true }).click();
  await page.waitForFunction(() => EditorView.findFromDOM(document.querySelector('.cm-editor')).state.doc.toString() === 'value = 1\nprint');
  await page.waitForFunction(() => languageMessages.some(message => message.method === 'textDocument/didChange' && message.params.contentChanges[0].text === 'value = 1\nprint'));
  const changed = await page.evaluate(() => languageMessages.filter(message => message.method === 'textDocument/didChange').at(-1));
  assert.equal(changed.params.contentChanges[0].text, 'value = 1\nprint');
  assert.equal(changed.params.contentChanges[0].range.start.line, 0);
  await page.locator('.cm-line').first().hover({ position: { x: 20, y: 10 } });
  await page.getByText('Fixture type information', { exact: true }).waitFor();
  await page.evaluate(() => closeEditor());
  await page.waitForFunction(() => languageSockets.every(socket => socket.readyState === 3));
  assert.ok(await page.evaluate(() => languageMessages.some(message => message.method === 'textDocument/didClose')));
});

test("a disconnected language server falls back without changing the document", async t => {
  const { page } = await openPage(t, true);
  await page.waitForFunction(() => document.querySelector('.editor-language-status')?.getAttribute("title")?.includes("LSP: fixture-lsp"));
  await page.evaluate(() => languageSockets[0].close());
  await page.waitForFunction(() => document.querySelector('.editor-language-status')?.getAttribute("title")?.includes("Internal fallback"));
  assert.equal(await page.locator('.cm-content').innerText(), 'value = 1\npri');
  assert.equal(await page.locator('.language-error').count(), 0);
});

test("compact LSP status follows UTF-8 and selects installed servers from the branch-style menu", async t => {
  const { page, rows } = await openPage(t, true);
  await page.waitForFunction(() => document.querySelector('.editor-language-status')?.getAttribute('title')?.includes('LSP: fixture-lsp'));
  const status = page.getByRole('button', { name: /Language server:/ });
  assert.equal(await status.innerText(), 'Python');
  assert.equal(await status.locator('svg').count(), 0);
  const alignment = await status.evaluate(button => {
    const label = button.querySelector('span'), encoding = button.closest('.editor-language-slot').previousElementSibling;
    const range = document.createRange();
    range.selectNodeContents(label); const text = range.getBoundingClientRect();
    range.selectNodeContents(encoding); const utf = range.getBoundingClientRect();
    const box = button.getBoundingClientRect();
    return { horizontal: Math.abs((text.left + text.right - box.left - box.right) / 2), vertical: Math.abs((text.top + text.bottom - utf.top - utf.bottom) / 2) };
  });
  assert.ok(alignment.horizontal <= 0.75, JSON.stringify(alignment));
  assert.ok(alignment.vertical <= 0.75, JSON.stringify(alignment));
  assert.equal(await page.locator('.editor-code-container .editor-language-status').count(), 0);
  assert.equal(await status.evaluate(button => button.closest('.editor-language-slot').previousElementSibling.textContent), 'UTF-8');
  await status.click();
  const menu = page.getByRole('listbox', { name: 'Language servers', exact: true });
  assert.equal(await menu.evaluate(element => element.classList.contains('editor-branch-menu')), true);
  assert.equal(await menu.locator('svg').count(), 0);
  const menuBox = await menu.boundingBox(), footerBox = await page.locator('.editor-status').boundingBox();
  assert.ok(menuBox.y + menuBox.height <= footerBox.y + 2);
  await menu.getByRole('option', { name: 'second-lsp', exact: true }).click();
  await page.waitForFunction(() => document.querySelector('.editor-language-status')?.getAttribute('title')?.includes('LSP: second-lsp'));
  assert.equal(rows.get('python').mode, 'custom');
  assert.deepEqual(rows.get('python').command, ['second-lsp', '--stdio']);
  await status.click();
  await menu.getByRole('option', { name: 'Internal', exact: true }).click();
  await page.waitForFunction(() => document.querySelector('.editor-language-status')?.getAttribute('title')?.includes('Python · Internal fallback'));
  assert.equal(await status.innerText(), 'Python');
  assert.equal(await status.locator('svg').count(), 0);
  assert.equal(rows.get('python').mode, 'fallback');
  await status.click();
  await menu.getByRole('option', { name: 'Automatic', exact: true }).click();
  await page.waitForFunction(() => document.querySelector('.editor-language-status')?.getAttribute('title')?.includes('LSP: fixture-lsp'));
  assert.equal(rows.get('python').mode, 'auto');
  await status.click();
  await page.keyboard.press('Escape');
  assert.equal(await menu.count(), 0);
});


test("without an installed LSP the menu only shows selected Internal even in automatic mode", async t => {
  const { page, rows } = await openPage(t);
  await page.waitForFunction(() => document.querySelector('.editor-language-status')?.getAttribute('title')?.includes('Python · Internal fallback'));
  assert.equal(rows.get('python').mode, 'auto');
  const status = page.getByRole('button', { name: /Language server:/ });
  await status.click();
  const menu = page.getByRole('listbox', { name: 'Language servers', exact: true });
  const internal = menu.getByRole('option', { name: 'Internal', exact: true });
  await internal.waitFor();
  assert.deepEqual(await menu.getByRole('option').allTextContents(), ['Internal']);
  assert.equal(await internal.getAttribute('aria-selected'), 'true');
  assert.equal(await menu.getByRole('option', { name: 'Automatic', exact: true }).count(), 0);
  assert.equal(await menu.locator('svg').count(), 0);
  await internal.click();
  await page.waitForFunction(() => document.querySelector('.editor-language-status')?.getAttribute('title')?.includes('Python · Internal fallback'));
  assert.equal(rows.get('python').mode, 'fallback');
});


test("an empty language registry still offers only selected Internal", async t => {
  const { page } = await openPage(t);
  await page.waitForFunction(() => document.querySelector('.editor-language-status')?.getAttribute('title')?.includes('Python · Internal fallback'));
  await page.route('**/__solomon/programming-languages', route => route.fulfill({ json: [] }));
  await page.getByRole('button', { name: /Language server:/ }).click();
  const menu = page.getByRole('listbox', { name: 'Language servers', exact: true });
  const internal = menu.getByRole('option', { name: 'Internal', exact: true });
  await internal.click();
  assert.equal(await menu.count(), 0);
  await page.getByRole('button', { name: /Language server:/ }).click();
  await internal.waitFor();
  assert.deepEqual(await menu.allTextContents(), ['Internal']);
  assert.equal(await internal.getAttribute('aria-selected'), 'true');
  assert.equal(await menu.locator('svg').count(), 0);
});
