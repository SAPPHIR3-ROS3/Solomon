import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";
import { stripTypeScriptTypes } from "node:module";
import { chromium } from "playwright-core";

test("explorer drag previews entries, moves into folders and rejects descendants", async () => {
  const source = await readFile(new URL("../src/editor/fileDrag.ts", import.meta.url), "utf8");
  const imports = await readFile(new URL("../src/editor/importFiles.ts", import.meta.url), "utf8");
  const code = stripTypeScriptTypes((imports + "\n" + source).replace(/^import .*;\n/gm, "").replace(/export /g, ""));
  const browser = await chromium.launch({ executablePath: "/usr/bin/chromium", headless: true, args: ["--no-sandbox"] });
  try {
    const page = await browser.newPage();
    await page.setContent(`<aside><header class="editor-root">Root</header><nav class="editor-file-list">
      <div class="right-side-panel-file"><button class="right-side-panel-file-row" title="a.txt">a.txt</button></div>
      <div class="right-side-panel-file"><button class="right-side-panel-file-row" title="folder">folder</button><div>
        <div class="right-side-panel-file"><button class="right-side-panel-file-row" title="nested">nested</button></div>
      </div></div></nav></aside>`);
    await page.evaluate(`(() => { ${code}
      window.moves = [];
      window.imports = [];
      window.nativeDrops = false;
      const NATIVE_FILE_DROP_EVENT = 'solomon:native-file-drop';
      const PROJECT_FILES_CHANGED_EVENT = 'solomon:project-files-changed';
      const usesNativeFileDrops = () => window.nativeDrops;
      const serverEndpoint = async path => path;
      window.fetch = async (_url, options) => { window.imports.push(JSON.parse(options.body)); return { ok: true }; };
      function useRef(value) { return { current: value }; }
      function useEffect(effect) { window.install = effect; }
      async function operateProjectFile(project, operation) { window.moves.push({ project, ...operation }); }
      const host = useFileDrag('project', { '': [{ name: 'a.txt', path: 'a.txt', isDirectory: false }, { name: 'folder', path: 'folder', isDirectory: true }], folder: [{ name: 'nested', path: 'folder/nested', isDirectory: true }] }, message => { throw Error(message); });
      host.current = document.querySelector('aside');
      window.dispose = window.install();
      window.fire = (title, type, external = false) => {
        const target = title === 'root' ? document.querySelector('.editor-root') : document.querySelector('[title="' + title + '"]');
        const dataTransfer = new DataTransfer();
        if (external) dataTransfer.items.add(new File([new Uint8Array([0, 255, 128])], 'external.bin'));
        const event = new DragEvent(type, { bubbles: true, cancelable: true, dataTransfer });
        target.dispatchEvent(event);
        return event.defaultPrevented;
      }; })()`);
    assert.equal(await page.locator('[title="a.txt"]').getAttribute("draggable"), "true");
    await page.evaluate(() => fire("a.txt", "dragstart"));
    assert.equal(await page.locator(".editor-file-drag-ghost").textContent(), "a.txt");
    assert.equal(await page.evaluate(() => fire("folder", "dragover")), true);
    assert.equal(await page.locator(".is-drop-target").getAttribute("title"), "folder");
    await page.evaluate(() => fire("folder", "drop"));
    assert.deepEqual(await page.evaluate(() => moves), [{ project: "project", action: "move", path: "a.txt", destination: "folder/a.txt" }]);
    assert.equal(await page.locator(".editor-file-drag-ghost").count(), 0);
    await page.evaluate(() => fire("folder", "dragstart"));
    assert.equal(await page.evaluate(() => fire("nested", "dragover")), false);
    await page.evaluate(() => fire("nested", "drop"));
    assert.equal(await page.evaluate(() => moves.length), 1);
    await page.evaluate(() => fire("nested", "dragstart"));
    await page.evaluate(() => fire("root", "drop"));
    assert.equal(await page.evaluate(() => moves[1].destination), "nested");
    assert.equal(await page.evaluate(() => fire("folder", "dragover", true)), true);
    assert.equal(await page.locator(".is-drop-target").getAttribute("title"), "folder");
    await page.evaluate(() => fire("folder", "drop", true));
    await page.waitForFunction(() => imports.length === 1);
    assert.deepEqual(await page.evaluate(() => imports[0]), { action: "import", path: "external.bin", destination: "folder/external.bin", content: "AP+A" });
    await page.evaluate(() => fire("root", "drop", true));
    await page.waitForFunction(() => imports.length === 2);
    assert.equal(await page.evaluate(() => imports[1].destination), "external.bin");
    await page.evaluate(() => { nativeDrops = true; fire("folder", "drop", true); });
    assert.equal(await page.evaluate(() => imports.length), 2);
    await page.evaluate(() => {
      const rect = document.querySelector('[title="folder"]').getBoundingClientRect();
      window.dispatchEvent(new CustomEvent('solomon:native-file-drop', { detail: { x: rect.x + 2, y: rect.y + 2, paths: ['/outside/report.pdf'] } }));
    });
    await page.waitForFunction(() => imports.length === 3);
    assert.deepEqual(await page.evaluate(() => imports[2]), { action: "import", path: "/outside/report.pdf", destination: "folder/report.pdf" });
    await page.evaluate(() => dispose());
  } finally {
    await browser.close();
  }
});
