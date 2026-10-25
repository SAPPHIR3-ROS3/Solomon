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
  entryDirectory = await mkdtemp(path.join(guiRoot, ".editor-images-test-"));
  fixture = await mkdtemp(path.join(tmpdir(), "solomon-editor-images-"));
  workspace = path.join(fixture, "a");
  projectA.path = workspace;
  projectB.path = path.join(fixture, "b");
  const home = path.join(fixture, "home");
  await mkdir(path.join(workspace, "src", "nested"), { recursive: true });
  await mkdir(projectB.path);
  await mkdir(home);
  await writeFile(path.join(workspace, "pixel.PNG"), Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a3ioAAAAASUVORK5CYII=", "base64"));
  await writeFile(path.join(workspace, "drawing.svg"), '<svg xmlns="http://www.w3.org/2000/svg" width="4000" height="3000"><rect width="4000" height="3000" fill="red"/><script>window.imageScriptExecuted = true</script></svg>');
  await writeFile(path.join(workspace, "broken.png"), "not an image");
  await writeFile(path.join(workspace, "src", "nested", "draft.txt"), "");
  await writeFile(path.join(projectB.path, "other.txt"), "Other project");
  await writeFile(path.join(home, "projectsId.json"), JSON.stringify({ [workspace]: projectA.id, [projectB.path]: projectB.id }));
  process.env.SOLOMON_HOME = home;
  loader = await createServer({ root: guiRoot, configFile: false, cacheDir: path.join(fixture, 'loader-cache'), server: { middlewareMode: true, hmr: false } });
  const { projectsPlugin } = await loader.ssrLoadModule("/projectsPlugin.ts");
  server = await createServer({ root: guiRoot, configFile: false, cacheDir: path.join(fixture, 'browser-cache'), plugins: [react(), projectsPlugin()], server: { host: "127.0.0.1", port: 0, hmr: false } });
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


async function openImage(page, name) {
  await page.locator(`#editor-files button[title="${name}"]`).click();
  await page.getByRole('region', { name: 'Image preview' }).waitFor();
}

test('image tabs display pixels, survive restoration and never save binary data as text', async () => {
  const original = await readFile(path.join(workspace, 'pixel.PNG'));
  const { page, context, errors } = await editorPage();
  const writes = [];
  page.on('request', request => { if (request.method() === 'PUT') writes.push(request.url()); });
  try {
    await openImage(page, 'pixel.PNG');
    await page.waitForFunction(() => document.querySelector('.editor-image-preview img')?.naturalWidth === 1);
    assert.equal(await page.locator('.cm-content').count(), 0);
    assert.equal(await page.getByRole('button', { name: 'Save file', exact: true }).isDisabled(), true);
    await page.keyboard.press('Control+s');
    await page.locator('#agent').click();
    await page.locator('#editor').click();
    await page.waitForFunction(() => document.querySelector('.editor-image-preview img')?.naturalWidth === 1);
    await page.reload();
    await page.waitForFunction(() => document.querySelector('.editor-image-preview img')?.naturalWidth === 1);
    assert.deepEqual(await readFile(path.join(workspace, 'pixel.PNG')), original);
    assert.deepEqual(writes, []);
    const stored = await page.evaluate(id => JSON.parse(localStorage.getItem(`solomon.editor-session.v1.${id}`)), projectA.id);
    assert.equal(stored.files[0].content, '');
    assert.equal(stored.files[0].saved, '');
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
});

test('SVG images fit the workbench without executing embedded scripts', async () => {
  const { page, context, errors } = await editorPage();
  try {
    await openImage(page, 'drawing.svg');
    await page.waitForFunction(() => document.querySelector('.editor-image-preview img')?.naturalWidth === 4000);
    const image = await page.locator('.editor-image-preview img').boundingBox();
    const preview = await page.locator('.editor-image-preview').boundingBox();
    assert.ok(image.width <= preview.width && image.height <= preview.height);
    assert.equal(await page.evaluate(() => window.imageScriptExecuted), undefined);
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
});

test('damaged images show an error rather than a text editor', async () => {
  const { page, context, errors } = await editorPage();
  try {
    await openImage(page, 'broken.png');
    await page.getByRole('alert').filter({ hasText: 'Unable to display this image' }).waitFor();
    assert.equal(await page.locator('.cm-content').count(), 0);
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
});
