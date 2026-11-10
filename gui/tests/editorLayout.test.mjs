import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";
import { chromium } from "playwright-core";

test("editor fills the workbench below the breadcrumb", async () => {
  const css = await readFile(new URL("../src/editor/editor.css", import.meta.url), "utf8");
  const browser = await chromium.launch({ executablePath: "/usr/bin/chromium", headless: true, args: ["--no-sandbox"] });
  try {
    const page = await browser.newPage();
    await page.setContent(`<style>*{box-sizing:border-box}${css}</style>
      <main class="editor-workbench" style="width:900px;height:800px">
        <div class="editor-tabs-shell">File</div>
        <div class="editor-breadcrumb">folder / file.py</div>
        <div class="editor-code-container"><div class="editor-code">Code</div><span class="editor-language-status">Python</span></div>
      </main>`);
    const breadcrumb = await page.locator(".editor-breadcrumb").boundingBox();
    const container = await page.locator(".editor-code-container").boundingBox();
    const code = await page.locator(".editor-code").boundingBox();
    assert.ok(container.y >= breadcrumb.y + breadcrumb.height);
    assert.equal(container.width, 900);
    assert.ok(code.height > 650, `Editor height: ${code.height}`);
  } finally {
    await browser.close();
  }
});
