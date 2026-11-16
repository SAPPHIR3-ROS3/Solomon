import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { stripTypeScriptTypes } from "node:module";
import { test } from "node:test";
import { chromium } from "playwright-core";

test("workspace drops reach a covered composer without triggering panel actions", async () => {
  const sources = await Promise.all(["fileReferences", "workspaceDrop"].map(name => readFile(new URL(`../src/chat/${name}.ts`, import.meta.url), "utf8")));
  const code = stripTypeScriptTypes(sources.join("\n").replace(/^import .*;\n/gm, "").replace(/export /g, ""));
  const browser = await chromium.launch({ executablePath: "/usr/bin/chromium", headless: true, args: ["--no-sandbox"] });
  try {
    const page = await browser.newPage();
    await page.setContent('<form style="position:fixed;left:100px;top:100px;width:300px;height:100px"><textarea></textarea></form><aside style="position:fixed;left:250px;top:0;width:250px;height:400px;z-index:5;background:white">Files</aside>');
    await page.evaluate(`(() => { ${code}
      window.references = [];
      window.panelDrops = 0;
      const form = document.querySelector('form');
      document.querySelector('aside').addEventListener('drop', () => panelDrops++);
      window.dispose = installWorkspaceDrop(form, 'project', reference => references.push(reference));
      window.fire = (type, x, y, projectID = 'project', mime = WORKSPACE_FILE_REFERENCE) => {
        const dataTransfer = new DataTransfer();
        dataTransfer.setData(mime, JSON.stringify({ projectID, path: 'src/file.ts' }));
        const target = document.elementFromPoint(x, y);
        const event = new DragEvent(type, { bubbles: true, cancelable: true, clientX: x, clientY: y, dataTransfer });
        target.dispatchEvent(event);
        return { prevented: event.defaultPrevented, target: target.tagName };
      };
    })()`);
    assert.deepEqual(await page.evaluate(() => fire('dragover', 300, 150)), { prevented: true, target: 'ASIDE' });
    assert.equal((await page.evaluate(() => fire('drop', 300, 150))).prevented, true);
    assert.deepEqual(await page.evaluate(() => references), [{ projectID: 'project', path: 'src/file.ts' }]);
    assert.equal(await page.evaluate(() => panelDrops), 0);
    await page.evaluate(() => fire('drop', 300, 150, 'other'));
    assert.equal(await page.evaluate(() => references.length), 1);
    assert.equal((await page.evaluate(() => fire('drop', 300, 250))).prevented, false);
    assert.equal((await page.evaluate(() => fire('drop', 300, 150, 'project', 'text/plain'))).prevented, false);
    await page.evaluate(() => document.querySelector('form').hidden = true);
    assert.equal((await page.evaluate(() => fire('drop', 300, 150))).prevented, false);
    await page.evaluate(() => { document.querySelector('form').hidden = false; dispose(); });
    assert.equal((await page.evaluate(() => fire('drop', 300, 150))).prevented, false);
  } finally {
    await browser.close();
  }
});
