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

function testPDF(pageCount) {
  const streamID = pageCount + 3;
  const objects = [
    '<< /Type /Catalog /Pages 2 0 R >>',
    `<< /Type /Pages /Kids [${Array.from({length:pageCount},(_,index)=>`${index+3} 0 R`).join(' ')}] /Count ${pageCount} >>`,
    ...Array.from({length:pageCount},()=>`<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 400] /Contents ${streamID} 0 R >>`),
    '<< /Length 23 >>\nstream\n1 0 0 rg 0 0 80 80 re f\nendstream',
  ];
  let pdf = '%PDF-1.4\n';
  const offsets = [0];
  for (const [index, object] of objects.entries()) { offsets.push(Buffer.byteLength(pdf)); pdf += `${index + 1} 0 obj\n${object}\nendobj\n`; }
  pdf += '%' + ' '.repeat(2 * 1024 * 1024) + '\n';
  const xref = Buffer.byteLength(pdf);
  pdf += `xref\n0 ${objects.length + 1}\n0000000000 65535 f \n` + offsets.slice(1).map(offset => String(offset).padStart(10, '0') + ' 00000 n \n').join('');
  return pdf + `trailer\n<< /Size ${objects.length + 1} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF`;
}

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
  await writeFile(path.join(workspace, 'large.PDF'), testPDF(2));
  await writeFile(path.join(workspace, 'book.pdf'), testPDF(41));
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


test('PDF uses partial requests, renders pages and remains read only after restoration', async () => {
  const { page, context, errors } = await editorPage();
  const ranges = [];
  const writes = [];
  page.on('request', request => {
    if (request.url().includes('large.PDF') && request.headers().range) ranges.push(request.headers().range);
    if (request.method() === 'PUT') writes.push(request.url());
  });
  try {
    await page.locator('#editor-files button[title="large.PDF"]').click();
    await page.getByRole('region', { name: 'PDF preview' }).waitFor();
    await page.waitForFunction(() => {
      const canvas = document.querySelector('.editor-pdf-pages canvas');
      return canvas?.width > 0 && canvas.style.visibility === 'visible';
    });
    const bounds = await page.evaluate(() => ({
      breadcrumb: document.querySelector('.editor-breadcrumb').getBoundingClientRect().toJSON(),
      toolbar: document.querySelector('.editor-pdf-toolbar').getBoundingClientRect().toJSON(),
      canvas: document.querySelector('canvas').getBoundingClientRect().toJSON(),
      preview: document.querySelector('.editor-pdf-preview').getBoundingClientRect().toJSON(),
    }));
    assert.ok(bounds.toolbar.top >= bounds.breadcrumb.bottom, 'PDF toolbar must sit below breadcrumb');
    assert.ok(bounds.canvas.top >= bounds.toolbar.bottom, 'PDF page must sit below toolbar');
    assert.ok(bounds.preview.height > 200, 'PDF preview must occupy the editor content row');
    assert.equal(await page.locator('.cm-content').count(), 0);
    assert.ok(ranges.length > 0, 'PDF should request byte ranges');
    const pixel = await page.locator('canvas').evaluate(canvas => Array.from(canvas.getContext('2d').getImageData(10, canvas.height - 10, 1, 1).data));
    assert.ok(pixel[0] > 200 && pixel[1] < 30, 'PDF page should contain rendered red rectangle');
    await page.getByRole('button', { name: 'Next page', exact: true }).click();
    await page.waitForFunction(() => document.querySelector('canvas')?.getAttribute('aria-label') === 'PDF page 2' && document.querySelector('canvas').style.visibility === 'visible');
    await page.reload();
    await page.waitForFunction(() => document.querySelector('canvas')?.style.visibility === 'visible');
    assert.equal(await page.getByRole('button', { name: 'Save file', exact: true }).isDisabled(), true);
    assert.deepEqual(writes, []);
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
});


test('server rejects text writes to PDFs and preserves original bytes', async () => {
  const original = await readFile(path.join(workspace, 'large.PDF'));
  const response = await fetch(url.replace('/editor-check', `/__solomon/projects/${projectA.id}/file?path=large.PDF`), {
    method: 'PUT', body: 'not PDF bytes', headers: { 'Content-Type': 'text/plain' },
  });
  assert.equal(response.status, 415);
  assert.deepEqual(await readFile(path.join(workspace, 'large.PDF')), original);
});

test('PDF zoom at scrollbar threshold settles without repeated renders or blank frames', async () => {
  const { page, context, errors } = await editorPage();
  try {
    await page.locator('#editor-files button[title="large.PDF"]').click();
    await page.waitForFunction(() => document.querySelector('canvas')?.style.visibility === 'visible');
    const heightAdjustment = await page.evaluate(() => {
      const host = document.querySelector('.editor-pdf-pages');
      const halfPageHeight = document.querySelector('canvas').getBoundingClientRect().height / 2;
      return halfPageHeight + 20 - (host.clientHeight - 40);
    });
    await page.setViewportSize({ width: 1200, height: Math.round(800 + heightAdjustment) });
    await page.evaluate(() => {
      const host = document.querySelector('.editor-pdf-pages');
      window.pdfBlankFrames = 0;
      window.pdfBlankObserver = new MutationObserver(() => {
        const canvas = host.querySelector('canvas');
        if (!canvas || getComputedStyle(canvas).visibility === 'hidden') window.pdfBlankFrames++;
      });
      window.pdfBlankObserver.observe(host, { childList: true, subtree: true, attributes: true });
    });
    await page.getByRole('button', { name: 'Zoom out', exact: true }).click();
    await page.getByRole('button', { name: 'Zoom out', exact: true }).click();
    await page.waitForFunction(() => document.querySelector('.editor-pdf-zoom')?.value === '50%' && document.querySelector('.editor-pdf-pages')?.getAttribute('aria-busy') === 'false');
    const stability = await page.evaluate(async () => {
      const host = document.querySelector('.editor-pdf-pages');
      let mutations = 0;
      const observer = new MutationObserver(records => { mutations += records.length; });
      observer.observe(host, { childList: true, subtree: true, attributes: true });
      await new Promise(resolve => setTimeout(resolve, 1500));
      observer.disconnect();
      return { mutations, visible: !!host.querySelector('canvas') && getComputedStyle(host.querySelector('canvas')).visibility === 'visible', error: host.querySelector('[role="alert"]')?.textContent };
    });
    const blankFrames = await page.evaluate(() => { window.pdfBlankObserver.disconnect(); return window.pdfBlankFrames; });
    assert.equal(blankFrames, 0, 'keep the previous PDF bitmap visible while zoom renders');
    assert.ok(stability.mutations <= 2, `zoom should settle, observed ${stability.mutations} DOM mutations`);
    assert.equal(stability.visible, true);
    assert.equal(stability.error, undefined);
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
});

test('PDF supports 25% zoom and fits the whole page after resizing', async () => {
  const { page, context, errors } = await editorPage();
  const waitForRender = () => page.waitForFunction(() => document.querySelector('canvas') && document.querySelector('.editor-pdf-pages')?.getAttribute('aria-busy') === 'false');
  const assertFits = async () => {
    const dimensions = await page.evaluate(() => {
      const host = document.querySelector('.editor-pdf-pages');
      const canvas = host.querySelector('canvas').getBoundingClientRect();
      return { width: canvas.width, height: canvas.height, availableWidth: host.clientWidth - 40, availableHeight: host.clientHeight - 40 };
    });
    assert.ok(dimensions.width <= dimensions.availableWidth + 1);
    assert.ok(dimensions.height <= dimensions.availableHeight + 1);
    assert.ok(Math.abs(dimensions.width - dimensions.availableWidth) < 2 || Math.abs(dimensions.height - dimensions.availableHeight) < 2, 'fit should use all available space along one axis: ' + JSON.stringify(dimensions));
  };
  try {
    await page.locator('#editor-files button[title="large.PDF"]').click();
    await waitForRender();
    const zoomOut = page.getByRole('button', { name: 'Zoom out', exact: true });
    for (let index = 0; index < 3; index++) await zoomOut.click();
    await waitForRender();
    assert.equal(await page.locator('.editor-pdf-zoom').inputValue(), '25%');
    assert.equal(await zoomOut.isDisabled(), true);
    const fit = page.locator('.editor-pdf-fit');
    await fit.click();
    await waitForRender();
    assert.equal(await fit.getAttribute('aria-label'), 'Fit to width');
    assert.equal(await fit.getAttribute('aria-pressed'), null);
    await assertFits();
    await page.setViewportSize({ width: 900, height: 560 });
    // ResizeObserver and rendering run after the viewport change is delivered.
    await page.waitForFunction(() => {
      const host = document.querySelector('.editor-pdf-pages');
      const canvas = host.querySelector('canvas').getBoundingClientRect();
      return host.getAttribute('aria-busy') === 'false' && canvas.width <= host.clientWidth - 39 && canvas.height <= host.clientHeight - 39;
    });
    await assertFits();
    await fit.click();
    await waitForRender();
    assert.equal(await fit.getAttribute('aria-label'), 'Fit to page');
    const widthFit = await page.evaluate(() => {
      const host = document.querySelector('.editor-pdf-pages');
      const canvas = host.querySelector('canvas').getBoundingClientRect();
      return { width: canvas.width, available: host.clientWidth - 40, height: canvas.height, availableHeight: host.clientHeight - 40 };
    });
    assert.ok(Math.abs(widthFit.width - widthFit.available) < 2, 'horizontal fit should fill the width');
    assert.ok(widthFit.height > widthFit.availableHeight, 'horizontal fit should allow vertical scrolling');
    await fit.click();
    await waitForRender();
    assert.equal(await fit.getAttribute('aria-label'), 'Fit to width');
    await assertFits();
    await page.getByRole('button', { name: 'Zoom in', exact: true }).click();
    await waitForRender();
    assert.equal(await fit.getAttribute('aria-label'), 'Fit to page');
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
});


test('PDF zoom accepts custom percentages, clamps limits and cancels invalid edits', async () => {
  const { page, context, errors } = await editorPage();
  try {
    await page.locator('#editor-files button[title="large.PDF"]').click();
    await page.waitForFunction(() => document.querySelector('canvas') && document.querySelector('.editor-pdf-pages')?.getAttribute('aria-busy') === 'false');
    await page.getByRole('button', { name: 'Fit to page', exact: true }).click();
    const zoom = page.getByRole('textbox', { name: 'Zoom percentage', exact: true });
    await zoom.fill('137%');
    await zoom.press('Enter');
    assert.equal(await zoom.inputValue(), '137%');
    await page.waitForFunction(() => Math.abs(document.querySelector('canvas').getBoundingClientRect().width - 548) < 1);
    assert.equal(await page.locator('.editor-pdf-fit').getAttribute('aria-label'), 'Fit to page');
    await zoom.fill('62,5');
    await zoom.press('Tab');
    await page.waitForFunction(() => Math.abs(document.querySelector('canvas').getBoundingClientRect().width - 250) < 1);
    await zoom.fill('10');
    await zoom.press('Enter');
    assert.equal(await zoom.inputValue(), '25%');
    await zoom.fill('999');
    await zoom.press('Enter');
    assert.equal(await zoom.inputValue(), '300%');
    await zoom.fill('invalid');
    await zoom.press('Tab');
    assert.equal(await zoom.inputValue(), '300%');
    await zoom.fill('88');
    await zoom.press('Escape');
    await zoom.press('Tab');
    assert.equal(await zoom.inputValue(), '300%');
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
});


test('two-page view navigates by pairs, fits both pages and handles an odd last page', async () => {
  const { page, context, errors } = await editorPage();
  try {
    await page.locator('#editor-files button[title="book.pdf"]').click();
    await page.waitForFunction(() => document.querySelector('canvas') && document.querySelector('.editor-pdf-pages').getAttribute('aria-busy') === 'false');
    const twoPages = page.getByRole('button', { name: 'Two-page view', exact: true });
    await twoPages.click();
    await page.waitForFunction(() => document.querySelectorAll('canvas').length === 2 && document.querySelector('.editor-pdf-pages').getAttribute('aria-busy') === 'false');
    const positions = await page.locator('canvas').evaluateAll(canvases => canvases.map(canvas => canvas.getBoundingClientRect().toJSON()));
    assert.equal(positions[0].top, positions[1].top);
    assert.ok(positions[1].left >= positions[0].right);
    await page.getByRole('button', { name: 'Fit to page', exact: true }).click();
    await page.waitForFunction(() => document.querySelector('.editor-pdf-pages').getAttribute('aria-busy') === 'false');
    const fit = await page.evaluate(() => {
      const host = document.querySelector('.editor-pdf-pages');
      const spread = host.querySelector('.editor-pdf-spread').getBoundingClientRect();
      return spread.width <= host.clientWidth - 39 && spread.height <= host.clientHeight - 39;
    });
    assert.equal(fit, true);
    await page.getByRole('button', { name: 'Next page', exact: true }).click();
    await page.waitForFunction(() => !!document.querySelector('canvas[aria-label="PDF page 3"]') && !!document.querySelector('canvas[aria-label="PDF page 4"]'));
    await page.getByRole('spinbutton', { name: 'PDF page', exact: true }).fill('41');
    await page.waitForFunction(() => !!document.querySelector('canvas[aria-label="PDF page 41"]'));
    assert.equal(await page.locator('canvas').count(), 1);
    assert.equal(await page.getByRole('button', { name: 'Next page', exact: true }).isDisabled(), true);
    await twoPages.click();
    await page.waitForFunction(() => document.querySelector('.editor-pdf-pages').getAttribute('aria-busy') === 'false');
    assert.equal(await twoPages.getAttribute('aria-pressed'), 'false');
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
});

test('continuous scrolling virtualizes pages and supports both axes with two-page view', async () => {
  const { page, context, errors } = await editorPage();
  try {
    await page.locator('#editor-files button[title="book.pdf"]').click();
    await page.waitForFunction(() => !!document.querySelector('canvas'));
    const continuous = page.getByRole('button', { name: 'Continuous scrolling', exact: true });
    const toolbarPositions = await page.locator('.editor-pdf-toolbar button').evaluateAll(buttons => buttons.map(button => ({ x: button.getBoundingClientRect().x, y: button.getBoundingClientRect().y })));
    assert.equal(await page.getByRole('button', { name: 'Horizontal scrolling', exact: true }).isDisabled(), true);
    await continuous.click();
    await page.waitForFunction(() => document.querySelector('.editor-pdf-flow.is-vertical canvas') && document.querySelector('.editor-pdf-pages').getAttribute('aria-busy') === 'false');
    assert.deepEqual(await page.locator('.editor-pdf-toolbar button').evaluateAll(buttons => buttons.map(button => ({ x: button.getBoundingClientRect().x, y: button.getBoundingClientRect().y }))), toolbarPositions, 'toolbar buttons must keep their positions when continuous scrolling is toggled');
    assert.equal(await page.locator('.editor-pdf-slot').count(), 41);
    assert.ok(await page.locator('canvas').count() < 10, 'offscreen pages must not allocate canvases');
    await page.getByRole('spinbutton', { name: 'PDF page', exact: true }).fill('35');
    await page.waitForFunction(() => !!document.querySelector('canvas[aria-label="PDF page 35"]'));
    await page.waitForFunction(() => !document.querySelector('canvas[aria-label="PDF page 1"]'));
    assert.ok(await page.locator('canvas').count() < 10);
    await page.getByRole('button', { name: 'Two-page view', exact: true }).click();
    await page.waitForFunction(() => !!document.querySelector('canvas[aria-label="PDF page 35"]') && !!document.querySelector('canvas[aria-label="PDF page 36"]'));
    await page.getByRole('button', { name: 'Horizontal scrolling', exact: true }).click();
    await page.waitForFunction(() => {
      const host = document.querySelector('.editor-pdf-pages');
      return !!host.querySelector('.editor-pdf-flow.is-horizontal canvas[aria-label="PDF page 35"]') && host.scrollWidth > host.clientWidth && host.scrollLeft > 0;
    });
    assert.equal(await page.getByRole('button', { name: 'Horizontal scrolling', exact: true }).getAttribute('aria-pressed'), 'true');
    const zoom = page.getByRole('textbox', { name: 'Zoom percentage', exact: true });
    await zoom.fill('75');
    await zoom.press('Enter');
    await page.waitForFunction(() => {
      const canvas = document.querySelector('canvas[aria-label="PDF page 35"]');
      return canvas && Math.abs(canvas.getBoundingClientRect().width - 300) < 1 && document.querySelector('[aria-label="PDF page"]').value === '35';
    });
    await page.getByRole('spinbutton', { name: 'PDF page', exact: true }).fill('1');
    await page.waitForFunction(() => document.querySelector('.editor-pdf-pages').scrollLeft < 2 && !!document.querySelector('canvas[aria-label="PDF page 1"]'));
    await page.locator('.editor-pdf-pages').dispatchEvent('wheel', { deltaY: 200, deltaX: 0, deltaMode: 0 });
    await page.waitForFunction(() => document.querySelector('.editor-pdf-pages').scrollLeft > 0);
    await page.locator('.editor-pdf-pages').evaluate(host => { host.scrollLeft = host.scrollWidth; });
    await page.waitForFunction(() => !!document.querySelector('canvas[aria-label="PDF page 41"]'));
    await page.waitForFunction(() => document.querySelector('[aria-label="PDF page"]').value === '41');
    assert.ok(await page.locator('canvas').count() < 10);
    await page.getByRole('button', { name: 'Vertical scrolling', exact: true }).click();
    await page.waitForFunction(() => !!document.querySelector('.editor-pdf-flow.is-vertical canvas[aria-label="PDF page 41"]'));
    await continuous.click();
    await page.waitForFunction(() => !document.querySelector('.editor-pdf-flow') && !!document.querySelector('canvas[aria-label="PDF page 41"]'));
    assert.equal(await page.getByRole('button', { name: 'Horizontal scrolling', exact: true }).isDisabled(), true);
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
});

test('PDF rendering is deduplicated, preloads two neighbors each way and survives bitmap eviction', async () => {
  const { page, context, errors } = await editorPage();
  try {
    const result = await page.evaluate(async () => {
      const { renderPdfSpread, prefetchPdfNeighbors } = await import('/src/editor/pdfRendering.ts');
      const { disposePdfPageCache } = await import('/src/editor/pdfPageCache.ts');
      const calls = {};
      const pages = new Map();
      const pdf = {
        numPages: 12,
        getPage: async number => {
          if (!pages.has(number)) pages.set(number, {
            getViewport: ({ scale }) => ({ width: 900 * scale, height: 1200 * scale }),
            cleanup: () => {},
            render: ({ canvas }) => {
              calls[number] = (calls[number] ?? 0) + 1;
              const context = canvas.getContext('2d');
              context.fillStyle = `rgb(${number * 10},0,0)`;
              context.fillRect(0, 0, canvas.width, canvas.height);
              return { promise: new Promise(resolve => setTimeout(resolve, 10)), cancel: () => {} };
            },
          });
          return pages.get(number);
        },
      };
      const size = { width: 3000, height: 3000 };
      const draw = numbers => renderPdfSpread(pdf, numbers, document, size, 1, null);
      const cancelled = draw([5]);
      const visible = draw([5]);
      cancelled.cancel();
      await Promise.all([cancelled.promise, visible.promise]);
      const stop = prefetchPdfNeighbors(pdf, 5, 1, document, size, 1, null);
      for (let attempt = 0; attempt < 200 && [3,4,6,7].some(number => !calls[number]); attempt++) await new Promise(resolve => setTimeout(resolve, 10));
      const prefetched = Object.keys(calls).map(Number).sort((a,b) => a-b);
      // A pair and a return visit reuse the individual page bitmaps.
      await draw([5,6]).promise;
      for (let number = 8; number <= 12; number++) await draw([number]).promise;
      const db = await new Promise((resolve, reject) => {
        const request = indexedDB.open('solomon-pdf-bitmaps', 1);
        request.onsuccess = () => resolve(request.result);
        request.onerror = () => reject(request.error);
      });
      const keys = () => new Promise((resolve, reject) => {
        const request = db.transaction('pages').objectStore('pages').getAllKeys();
        request.onsuccess = () => resolve(request.result);
        request.onerror = () => reject(request.error);
      });
      let stored = [];
      for (let attempt = 0; attempt < 200; attempt++) {
        stored = await keys();
        if (stored.length >= 10) break;
        await new Promise(resolve => setTimeout(resolve, 20));
      }
      const revisited = await draw([5]).promise;
      const canvas = revisited.element.querySelector('canvas');
      const pixel = canvas.getContext('2d').getImageData(10,10,1,1).data[0];
      stop();
      disposePdfPageCache(pdf);
      let remaining = stored;
      for (let attempt = 0; attempt < 200 && remaining.length; attempt++) {
        remaining = (await keys()).filter(key => stored.includes(key));
        if (remaining.length) await new Promise(resolve => setTimeout(resolve, 10));
      }
      db.close();
      return { calls, prefetched, pixel, persisted: stored.length, remaining: remaining.length };
    });
    assert.deepEqual(result.prefetched, [3,4,5,6,7]);
    assert.ok(Object.values(result.calls).every(count => count === 1), 'each page should render once, including when its bitmap is restored from disk');
    assert.ok(result.persisted >= 10, 'compressed page bitmaps should be stored outside the hot bitmap cache');
    assert.equal(result.pixel, 50);
    assert.equal(result.remaining, 0, 'closing the PDF should delete its temporary bitmap cache');
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
});

test('scanned grayscale images declared as RGB retain correct pixels without buffer overruns', async () => {
  const { page, context } = await editorPage();
  try {
    const result = await page.evaluate(async () => {
      const { normalizePdfRaster } = await import('/src/editor/pdfCompatibility.ts');
      const grayscale = { width: 2, height: 2, kind: 2, data: new Uint8Array([0, 64, 180, 255]) };
      normalizePdfRaster(grayscale);
      const rgb = { width: 1, height: 1, kind: 2, data: new Uint8Array([15, 30, 45]) };
      normalizePdfRaster(rgb);
      return { grayscale: Array.from(grayscale.data), kind: grayscale.kind, rgb: Array.from(rgb.data), rgbKind: rgb.kind };
    });
    assert.deepEqual(result.grayscale, [0,0,0,255,64,64,64,255,180,180,180,255,255,255,255,255]);
    assert.equal(result.kind, 3);
    assert.deepEqual(result.rgb, [15,30,45]);
    assert.equal(result.rgbKind, 2);
  } finally { await context.close(); }
});

test('changing continuous scroll direction preserves the current canvas without blank frames', async () => {
  const { page, context, errors } = await editorPage();
  try {
    await page.locator('#editor-files button[title="book.pdf"]').click();
    await page.waitForFunction(() => !!document.querySelector('canvas'));
    await page.getByRole('button', { name: 'Two-page view', exact: true }).click();
    await page.getByRole('button', { name: 'Continuous scrolling', exact: true }).click();
    await page.getByRole('spinbutton', { name: 'PDF page', exact: true }).fill('15');
    await page.waitForFunction(() => document.querySelector('canvas[aria-label="PDF page 15"]') && document.querySelector('.editor-pdf-pages').getAttribute('aria-busy') === 'false');
    await page.evaluate(() => {
      window.directionCanvas = document.querySelector('canvas[aria-label="PDF page 15"]');
      window.directionBlankFrames = 0;
      window.directionObserver = new MutationObserver(() => {
        if (!window.directionCanvas.isConnected) window.directionBlankFrames++;
      });
      window.directionObserver.observe(document.querySelector('.editor-pdf-pages'), { subtree: true, childList: true });
    });
    for (const direction of ['Horizontal scrolling', 'Vertical scrolling', 'Horizontal scrolling', 'Vertical scrolling']) {
      await page.getByRole('button', { name: direction, exact: true }).click();
      await page.waitForFunction(direction => {
        const host = document.querySelector('.editor-pdf-pages');
        return host.classList.contains(direction.startsWith('Horizontal') ? 'is-horizontal' : 'is-vertical') && host.getAttribute('aria-busy') === 'false';
      }, direction);
      // Allow layout, observers and cached render promises to settle.
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      assert.equal(await page.evaluate(() => document.querySelector('canvas[aria-label="PDF page 15"]') === window.directionCanvas), true, 'direction changes must reuse the same displayed canvas');
      assert.equal(await page.getByRole('spinbutton', { name: 'PDF page', exact: true }).inputValue(), '15');
    }
    const blankFrames = await page.evaluate(() => { window.directionObserver.disconnect(); return window.directionBlankFrames; });
    assert.equal(blankFrames, 0);
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
});
