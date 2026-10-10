import assert from "node:assert/strict";
import { test } from "node:test";
import { existsSync } from "node:fs";
import { mkdtemp, mkdir, rm, stat } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import react from "@vitejs/plugin-react";
import { chromium } from "playwright-core";

test("folder creation stays visible with short, long and filtered lists", async () => {
 const root = fileURLToPath(new URL("..", import.meta.url));
 const fixture = await mkdtemp(path.join(tmpdir(), "solomon-folder-picker-"));
 const loader = await createServer({ root, configFile: false, server: { middlewareMode: true, hmr: false } });
 let server, browser;
 try {
  const { projectsPlugin } = await loader.ssrLoadModule("/projectsPlugin.ts");
  const fixturePlugin = { name: "folder-picker-fixture", configureServer(vite) {
    vite.middlewares.use("/__folder-test", async (_req, res) => {
      res.setHeader("Content-Type", "text/html");
      res.end(await vite.transformIndexHtml("/__folder-test", `<html><body style="margin:0"><div id="root"></div><script type="module">
        import React from 'react';
        import {createRoot} from 'react-dom/client';
        import {NewProjectDialog} from '/src/projects/NewProjectDialog.tsx';
        createRoot(document.getElementById('root')).render(React.createElement(NewProjectDialog,{isOpen:true,onClose:()=>{},onConfirmLocalFolder:()=>{}}));
      </script></body></html>`));
    });
  } };
  server = await createServer({ root, configFile: false, plugins: [react(), projectsPlugin(), fixturePlugin], server: { host: "127.0.0.1", port: 0, hmr: false } });
  await server.listen();
  const origin = `http://127.0.0.1:${server.httpServer.address().port}`;
  const endpoint = `${origin}/__solomon/filesystem-directories`;
  const create = (name) => fetch(endpoint, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ path: fixture, name }) });
  assert.equal((await create("API folder")).status, 201);
  assert.equal((await create("API folder")).status, 400);
  for (const name of ["..", "../escape", "nested/folder", "nested\\folder", ""]) assert.equal((await create(name)).status, 400);
  browser = await chromium.launch({ executablePath: ["/usr/bin/chromium", "/usr/bin/chromium-browser", "/usr/bin/google-chrome"].find(existsSync), headless: true, args: ["--no-sandbox"] });
  const page = await browser.newPage();
  page.on("pageerror", (error) => console.error(error.message));
  page.on("console", (message) => { if (message.type() === "error") console.error(message.text()); });
  await page.goto(`${origin}/__folder-test`);
  page.setDefaultTimeout(5000);
  await page.getByRole("button", { name: /Local folder/ }).click();
  const input = page.getByRole("textbox", { name: "Folder path or search folders" });
  async function checkVisible() {
   const button = page.getByRole("button", { name: "Create folder", exact: true });
   await button.waitFor({ state: "visible" });
   const box = await button.boundingBox();
   const viewport = page.viewportSize();
   assert.ok(box.y >= 0 && box.y + box.height <= viewport.height);
   assert.ok(await button.evaluate((element) => !element.closest('.new-project-dialog-folder-list')));
  }
  await input.fill(`${fixture}/`);
  await page.getByRole("button", { name: "API folder", exact: true }).waitFor();
  await checkVisible();
  for (let i = 0; i < 25; i++) await mkdir(path.join(fixture, `Folder ${i}`));
  await input.fill(`${fixture}/API folder/`);
  await page.getByRole("button", { name: "Go to parent folder" }).click();
  await page.getByRole("button", { name: "Folder 24", exact: true }).waitFor();
  for (const viewport of [{ width: 1280, height: 800 }, { width: 390, height: 350 }]) {
   await page.setViewportSize(viewport);
   await checkVisible();
   assert.ok(await page.locator('.new-project-dialog-folder-list').evaluate((el) => el.scrollHeight > el.clientHeight));
   await page.locator('.new-project-dialog-folder-list').evaluate((el) => { el.scrollTop = el.scrollHeight; });
   await checkVisible();
  }
  await input.fill(`${fixture}/Created from picker`);
  await checkVisible();
  await page.getByRole("button", { name: "Create folder", exact: true }).click();
  assert.equal(await page.getByRole("textbox", { name: "New folder name" }).inputValue(), "Created from picker");
  await page.getByRole("button", { name: "Create", exact: true }).click();
  await page.waitForFunction(() => document.querySelector('[aria-label="Folder path or search folders"]').value.endsWith('/Created from picker'));
  assert.ok((await stat(path.join(fixture, "Created from picker"))).isDirectory());
 } finally {
  await browser?.close(); await server?.close(); await loader.close(); await rm(fixture, { recursive: true, force: true });
 }
});
