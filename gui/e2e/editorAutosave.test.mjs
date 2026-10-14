import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import react from "@vitejs/plugin-react";
import { chromium } from "playwright-core";

const guiRoot = fileURLToPath(new URL("..", import.meta.url));
const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/chromium-browser", "/usr/bin/google-chrome"].find(existsSync);
let fixture, entryDirectory, workspace, loader, server, browser, url;
const previousHome = process.env.SOLOMON_HOME;
const projectA = { id: "a".repeat(64), name: "Project A", chats: [], chatCount: 0 };
const projectB = { id: "b".repeat(64), name: "Project B", chats: [], chatCount: 0 };

before(async () => {
  entryDirectory = await mkdtemp(path.join(guiRoot, ".editor-autosave-test-"));
  fixture = await mkdtemp(path.join(tmpdir(), "solomon-editor-autosave-"));
  workspace = path.join(fixture, "a");
  projectA.path = workspace;
  projectB.path = path.join(fixture, "b");
  const home = path.join(fixture, "home");
  await mkdir(path.join(workspace, "src", "nested"), { recursive: true });
  await mkdir(projectB.path);
  await mkdir(home);
  await writeFile(path.join(workspace, "src", "nested", "draft.txt"), "");
  await writeFile(path.join(projectB.path, "other.txt"), "Other project");
  await writeFile(path.join(home, "projectsId.json"), JSON.stringify({ [workspace]: projectA.id, [projectB.path]: projectB.id }));
  process.env.SOLOMON_HOME = home;
  loader = await createServer({ root: guiRoot, configFile: false, server: { middlewareMode: true, hmr: false } });
  const { projectsPlugin } = await loader.ssrLoadModule("/projectsPlugin.ts");
  server = await createServer({ root: guiRoot, configFile: false, plugins: [react(), projectsPlugin()], server: { host: "127.0.0.1", port: 0 } });
  await server.listen();
  url = `http://127.0.0.1:${server.httpServer.address().port}/editor-check`;
  browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
});

after(async () => {
  await browser?.close();
  await server?.close();
  await loader?.close();
  if (previousHome === undefined) delete process.env.SOLOMON_HOME;
  else process.env.SOLOMON_HOME = previousHome;
  if (entryDirectory) await rm(entryDirectory, { recursive: true, force: true });
  if (fixture) await rm(fixture, { recursive: true, force: true });
});

async function editorPage() {
  const context = await browser.newContext({ viewport: { width: 1200, height: 800 } });
  const page = await context.newPage();
  page.setDefaultTimeout(7000);
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.route("**/__solomon/**", (route) => {
    const pathname = new URL(route.request().url()).pathname;
    if (["/status", "/history", "/branches"].some((suffix) => pathname.endsWith(suffix))) {
      return route.fulfill({ json: { isRepo: false, changes: {}, staged: {}, branches: [], commits: [] } });
    }
    return route.continue();
  });
  const entry = `
    import React from 'react';
    import { createRoot } from 'react-dom/client';
    import { EditorPage } from '/src/editor/EditorPage.tsx';
    import '/src/styles.css'; import '/src/editor/editor.css';
    function Harness() {
      const [visible, setVisible] = React.useState(true);
      const [project, setProject] = React.useState(${JSON.stringify(projectA)});
      return React.createElement(React.Fragment, null,
        React.createElement('nav', { style: {position:'fixed',zIndex:100,top:0,right:0} },
          React.createElement('button', {id:'agent',onClick:()=>setVisible(false)}, 'Agent'),
          React.createElement('button', {id:'editor',onClick:()=>setVisible(true)}, 'Editor'),
          React.createElement('button', {id:'project-a',onClick:()=>setProject(${JSON.stringify(projectA)})}, 'Project A'),
          React.createElement('button', {id:'project-b',onClick:()=>setProject(${JSON.stringify(projectB)})}, 'Project B')),
        visible ? React.createElement(EditorPage, { key:project.id, project, bottomInset:0, onHome:()=>{} }) : null);
    }
    createRoot(document.getElementById('root')).render(React.createElement(React.StrictMode,null,React.createElement(Harness)));
  `;
  await writeFile(path.join(entryDirectory, "entry.jsx"), entry);
  const html = await server.transformIndexHtml("/editor-check", `<div id="root" class="app-shell" data-active-view="editor"></div><script type="module" src="/${path.basename(entryDirectory)}/entry.jsx"></script>`);
  await page.route("**/editor-check", (route) => route.fulfill({ contentType: "text/html", body: html }));
  await page.goto(url);
  try { await page.locator(".editor-root-identity").waitFor(); }
  catch (reason) { await context.close(); throw new Error(`${reason.message}\nBrowser errors: ${JSON.stringify(errors)}`); }
  return { page, context, errors };
}

const filePath = () => path.join(workspace, "src", "nested", "draft.txt");
async function openDraft(page) {
  await page.locator('#editor-files button[title="src"]').click();
  await page.locator('#editor-files button[title="nested"]').click();
  await page.locator('#editor-files button[title="draft.txt"]').click();
  await page.locator(".cm-content").waitFor();
}
async function typeText(page, content) {
  await page.locator(".cm-content").click();
  await page.keyboard.press("Control+a");
  await page.keyboard.insertText(content);
}
async function waitForDisk(content) {
  for (let attempt = 0; attempt < 100; attempt++) {
    if (await readFile(filePath(), "utf8") === content) return;
    await new Promise((resolve) => setTimeout(resolve, 30));
  }
  assert.equal(await readFile(filePath(), "utf8"), content);
}

test("autosave and immediate Agent switch preserve text and open folders", async () => {
  await writeFile(filePath(), "");
  const { page, context, errors } = await editorPage();
  try {
    await openDraft(page);
    await typeText(page, "Autosaved draft");
    await waitForDisk("Autosaved draft");
    await typeText(page, "Latest text before switching");
    await page.locator("#agent").click();
    await page.locator("#editor").click();
    await page.waitForFunction(() => document.querySelector(".cm-content")?.textContent === "Latest text before switching");
    await waitForDisk("Latest text before switching");
    assert.equal(await page.locator('#editor-files button[title="src"]').getAttribute("aria-expanded"), "true");
    assert.equal(await page.locator('#editor-files button[title="nested"]').getAttribute("aria-expanded"), "true");
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
});

test("sessions are separate per project and restore after a page reload", async () => {
  await writeFile(filePath(), "Saved text");
  const { page, context, errors } = await editorPage();
  try {
    await openDraft(page);
    await page.locator("#project-b").click();
    assert.equal(await page.locator(".cm-content").count(), 0);
    await page.locator('#editor-files button[title="other.txt"]').click();
    await page.waitForFunction(() => document.querySelector(".cm-content")?.textContent === "Other project");
    await page.locator("#project-a").click();
    await page.waitForFunction(() => document.querySelector(".cm-content")?.textContent === "Saved text");
    await writeFile(filePath(), "Changed on disk");
    await page.reload();
    await page.waitForFunction(() => document.querySelector(".cm-content")?.textContent === "Changed on disk");
    assert.equal(await page.locator('#editor-files button[title="nested"]').getAttribute("aria-expanded"), "true");
    await page.getByRole("button", { name: "Project A folder" }).click();
    await page.locator("#agent").click(); await page.locator("#editor").click();
    assert.equal(await page.getByRole("button", { name: "Project A folder" }).getAttribute("aria-expanded"), "false");
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
});

test("failed autosaves retain a draft across unmount and reload", async () => {
  await writeFile(filePath(), "");
  const { page, context, errors } = await editorPage();
  try {
    await page.route("**/__solomon/projects/*/file?*", (route) => route.request().method() === "PUT" ? route.fulfill({ status: 500, body: "Save unavailable" }) : route.continue());
    await openDraft(page);
    await typeText(page, "Recover this unsaved text");
    await page.getByText("Autosave failed.", { exact: false }).waitFor();
    await page.locator("#agent").click(); await page.locator("#editor").click();
    await page.waitForFunction(() => document.querySelector(".cm-content")?.textContent === "Recover this unsaved text");
    await page.reload();
    await page.waitForFunction(() => document.querySelector(".cm-content")?.textContent === "Recover this unsaved text");
    assert.equal(await readFile(filePath(), "utf8"), "");
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
});

test("a delayed save cannot overwrite newer edits after a view switch", async () => {
  await writeFile(filePath(), "");
  const { page, context, errors } = await editorPage();
  let release;
  const gate = new Promise((resolve) => { release = resolve; });
  let started;
  const firstStarted = new Promise((resolve) => { started = resolve; });
  const writes = [];
  try {
    await page.route("**/__solomon/projects/*/file?*", async (route) => {
      if (route.request().method() === "PUT") {
        const content = route.request().postData();
        writes.push(content);
        if (content === "First edit") { started(); await gate; }
      }
      await route.continue();
    });
    await openDraft(page);
    await typeText(page, "First edit");
    await firstStarted;
    await typeText(page, "Newer edit");
    await page.locator("#agent").click(); await page.locator("#editor").click();
    await page.waitForFunction(() => document.querySelector(".cm-content")?.textContent === "Newer edit");
    release();
    await waitForDisk("Newer edit");
    assert.equal(writes[0], "First edit");
    assert.equal(writes.at(-1), "Newer edit");
    assert.deepEqual(errors, []);
  } finally { release(); await context.close(); }
});

test("Enter inserts one newline and persists it across reopening", async () => {
  await writeFile(filePath(), "");
  const { page, context, errors } = await editorPage();
  try {
    await openDraft(page);
    await typeText(page, "First line");
    await page.evaluate(() => window.addEventListener("keydown", event => {
      if (event.key === "Enter") window.editorEnterPrevented = event.defaultPrevented;
    }));
    await page.keyboard.press("Enter");
    assert.equal(await page.evaluate(() => window.editorEnterPrevented), true);
    await page.keyboard.insertText("Second line");
    const content = await page.evaluate((id) => JSON.parse(localStorage.getItem(`solomon.editor-session.v1.${id}`)).files[0].content, projectA.id);
    assert.equal(content, "First line\nSecond line");
    await waitForDisk(content);
    await page.keyboard.press("Enter");
    await page.keyboard.press("Shift+Enter");
    await page.keyboard.insertText("Fourth line");
    const finalContent = "First line\nSecond line\n\nFourth line";
    await waitForDisk(finalContent);
    await page.locator("#agent").click();
    await page.locator("#editor").click();
    await page.waitForFunction((expected) => Array.from(document.querySelectorAll(".cm-line")).map(line => line.textContent).join("\n") === expected, finalContent);
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
});

test("editor highlights language files using the Cursor Dark TextMate colors", async () => {
  const examples = [
    { name: "highlight.ts", source: 'const answer = 42;\nconst label = "hello"; // note\n', token: "const", color: "rgb(130, 210, 206)" },
    { name: "highlight.py", source: 'def greet():\n    return "hello" # note\n', token: "def", color: "rgb(130, 210, 206)" },
    { name: "highlight.toml", source: '# note\nname = "hello"\ncount = 42\n', token: "42", color: "rgb(235, 200, 141)" },
    { name: "highlight.json", source: '{"count": 42, "label": "hello"}\n', token: "42", color: "rgb(235, 200, 141)" },
  ];
  for (const example of examples) await writeFile(path.join(workspace, example.name), example.source);
  const { page, context, errors } = await editorPage();
  try {
    for (const example of examples) {
      await page.locator(`#editor-files button[title="${example.name}"]`).click();
        try { await page.waitForFunction(({ token, color }) => Array.from(document.querySelectorAll(".cm-content span")).some(span => span.textContent === token && getComputedStyle(span).color === color), example); }
      catch (error) { throw new Error(`${example.name}: ${error.message}; tokens=${JSON.stringify(await page.locator('.cm-content').evaluate(node => [...node.querySelectorAll('span')].map(span => ({ text: span.textContent, color: getComputedStyle(span).color }))))}`); }
      assert.equal(await readFile(path.join(workspace, example.name), "utf8"), example.source);
    }
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
});

test("Enter retains indentation and brackets auto-close, skip and delete as pairs", async () => {
  await writeFile(filePath(), "");
  const { page, context, errors } = await editorPage();
  try {
    await openDraft(page);
    await typeText(page, "    first");await page.keyboard.press("Enter");await page.keyboard.insertText("second");
    await waitForDisk("    first\n    second");
    await typeText(page, "\tfirst");await waitForDisk("\tfirst");
    await page.locator('#agent').click();await page.locator('#editor').click();
    await page.locator('.cm-content').click();await page.keyboard.press('Control+End');
    await page.keyboard.press('Enter');await page.keyboard.insertText('second');await waitForDisk("\tfirst\n\tsecond");
    for (const [open, close] of [['(',')'],['[',']'],['{','}'],['"','"'],["'","'"]]) {
      await typeText(page, "");await page.keyboard.type(open);await waitForDisk(open+close);
      await page.keyboard.type(close);await waitForDisk(open+close);
      await page.keyboard.press('ArrowLeft');await page.keyboard.press('Backspace');await waitForDisk("");
    }
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
});

test("editor tabs restore their own scroll and cursor across Agent switches and reload", async () => {
  const text=Array.from({length:260},(_,index)=>`Line ${index+1}`).join('\n');
  await writeFile(filePath(),text);await writeFile(path.join(workspace,'another.txt'),text);
  const {page,context,errors}=await editorPage();
  try {
    await openDraft(page);await page.locator('.cm-content').click();await page.keyboard.press('Control+End');
    const shell=page.locator('.cm-scroller');
    await page.waitForFunction(()=>document.querySelector('.cm-scroller').scrollTop>2000 && document.querySelector('.editor-status').textContent.includes('Ln 260'));
    await page.evaluate(()=>document.fonts.ready);
    const settle=()=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))));
    await shell.evaluate(node=>{node.scrollTop=1300});await settle();
    const top=await shell.evaluate(node=>node.scrollTop);
    const before = await page.evaluate(id=>JSON.parse(localStorage.getItem(`solomon.editor-session.v1.${id}`)),projectA.id);
    await page.locator('#agent').click();
    const hidden = await page.evaluate(id=>JSON.parse(localStorage.getItem(`solomon.editor-session.v1.${id}`)),projectA.id);
    await page.locator('#editor').click();
    try { await page.waitForFunction(top=>Math.abs(document.querySelector('.cm-scroller').scrollTop-top)<2,top); }
    catch(error){throw new Error(error.message+'; top='+top+'; before='+JSON.stringify(before.views)+'; hidden='+JSON.stringify(hidden.views)+'; now='+JSON.stringify(await page.evaluate(id=>({top:document.querySelector('.cm-scroller')?.scrollTop,session:JSON.parse(localStorage.getItem(`solomon.editor-session.v1.${id}`))?.views}),projectA.id)));}
    assert.ok((await page.locator('.editor-status').innerText()).includes('Ln 260'));
    await page.locator('#editor-files button[title="another.txt"]').dblclick();
    await page.waitForFunction(()=>document.querySelector('.editor-breadcrumb')?.textContent.includes('another.txt'));
    await shell.evaluate(node=>{node.scrollTop=700});await settle();
    const topB=await shell.evaluate(node=>node.scrollTop);
    await page.locator('.editor-tab-trigger').filter({hasText:'draft.txt'}).click();
    await page.waitForFunction(top=>Math.abs(document.querySelector('.cm-scroller').scrollTop-top)<2,top);
    await page.locator('.editor-tab-trigger').filter({hasText:'another.txt'}).click();
    await page.waitForFunction(top=>Math.abs(document.querySelector('.cm-scroller').scrollTop-top)<2,topB);
    await page.reload();await page.waitForFunction(top=>Math.abs(document.querySelector('.cm-scroller')?.scrollTop-top)<2,topB);
    assert.deepEqual(errors,[]);
  } finally {await context.close();}
});
