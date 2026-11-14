import assert from "node:assert/strict";
import { test } from "node:test";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { chromium } from "playwright-core";

test("Markdown defaults to preview and switches to editable source", async () => {
  const fixture = `import React, {useState} from 'react';
    import {createRoot} from 'react-dom/client';
    import {DocumentView} from '/src/editor/DocumentView.tsx';
    import '/src/editor/editor.css';
    import '/src/typography.css';
    import '/src/theme/themes.css';
    import {MarkdownContent} from '/src/chat/MarkdownContent.tsx';
    function App() {
      const [toolbarHost, setToolbarHost] = useState(null);
      const [content, setContent] = useState('# Hello\\n\\n**Markdown**');
      const [path, setPath] = useState('README.MD');
      const [comparison, setComparison] = useState(false);
      window.changeFile = setPath;
      window.compareMarkdown = text => { setContent(text); setPath("comparison.md"); setComparison(true); };
      return <><div className="editor-breadcrumb"><span>{path}</span><div className="editor-document-controls" ref={setToolbarHost}/></div><DocumentView toolbarHost={toolbarHost} key={path} file={{id:path,path,content,saved:content}} projectID="test" onChange={setContent} onCursor={()=>{}} onSave={()=>{}} onViewState={()=>{}}/>{comparison ? <article id="chat-reference" className="chat-message is-assistant"><div className="chat-response-content"><MarkdownContent content={content}/></div></article> : null}</>;
    }
    createRoot(document.getElementById('root')).render(<App/>);`;
  const cacheDir = await mkdtemp(join(tmpdir(), "solomon-markdown-editor-"));
  const server = await createServer({
    root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, cacheDir,
    esbuild: { jsx: "automatic" }, server: { host: "127.0.0.1", port: 0, hmr: false },
    plugins: [{
      name: "markdown-editor-fixture",
      resolveId(id, importer) {
        if (id === "/fixture.tsx") return id;
      },
      load(id) {
        if (id === "/fixture.tsx") return fixture;
      },
      configureServer(server) {
        server.middlewares.use((req, res, next) => {
          if (req.url !== "/fixture") return next();
          res.setHeader("Content-Type", "text/html");
          res.end('<main class="editor-workbench" style="width:900px;height:800px"><div></div><div></div><div id="root" style="display:contents"></div></main><script type="module" src="/fixture.tsx"></script>');
        });
      },
    }],
  });
  let browser;
  try {
    await server.listen();
    browser = await chromium.launch({ executablePath: "/usr/bin/chromium", headless: true, args: ["--no-sandbox"] });
    const page = await browser.newPage();
    await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`);
    await page.getByRole("heading", { name: "Hello" }).waitFor();
    assert.equal(await page.getByRole("button", { name: "Preview", exact: true }).getAttribute("aria-pressed"), "true");
    assert.equal(await page.getByRole("textbox").count(), 0);
    assert.equal(await page.locator(".editor-breadcrumb").getByRole("group", { name: "Document view" }).count(), 1);
    assert.ok((await page.locator(".editor-document").boundingBox()).height > 650);
    await page.getByRole("button", { name: "Source", exact: true }).click();
    assert.ok((await page.locator(".editor-code-container").boundingBox()).height > 600);
    await page.getByRole("textbox").fill("# Changed\n\nNew content");
    await page.getByRole("button", { name: "Preview", exact: true }).click();
    await page.getByRole("heading", { name: "Changed" }).waitFor();
    await page.getByRole("button", { name: "Source", exact: true }).click();
    assert.equal(await page.getByRole("textbox").textContent(), "# ChangedNew content");
    await page.evaluate(() => changeFile("notes.markdown"));
    await page.getByRole("heading", { name: "Changed" }).waitFor();
    await page.evaluate(() => changeFile("main.ts"));
    await page.getByRole("textbox").waitFor();
    assert.equal(await page.getByRole("group", { name: "Document view" }).count(), 0);
    await page.evaluate(() => compareMarkdown([
      '# Heading', '## Subheading', 'Paragraph with **bold**, *emphasis*, `inline code` and [link](https://example.com).',
      '| Name | Value |', '| --- | --- |', '| First | A long value |', '',
      '> A block quote', '', '- List item', '- Second item', '', '- [x] Done', '- [ ] Pending', '',
      '1. Ordered item', '', '---', '', '```js', 'const value = 1;', '```', '', '$x^2$', '',
      '```mermaid', 'flowchart LR', 'A --> B', '```',
    ].join('\n')));
    await page.locator('.editor-document-preview table').waitFor();
    const selectors = ['h1', 'h2', 'p', 'strong', 'em', 'a', 'ul', 'ol', 'li', 'blockquote', 'table', 'th', 'td', 'p > code', 'pre', '.chat-code-block', '.chat-code-toolbar', '.chat-code-line', '.chat-line-number', '.chat-line-content', '.chat-checkbox', 'hr', '.katex', '.chat-mermaid-preview'];
    const properties = ['fontFamily', 'fontSize', 'fontWeight', 'lineHeight', 'color', 'backgroundColor', 'border', 'borderRadius', 'padding', 'margin', 'display', 'tableLayout', 'textAlign', 'whiteSpace'];
    for (const selector of selectors) {
      const styles = await page.evaluate(({selector, properties}) => {
        return ['.editor-document-preview', '#chat-reference'].map(root => {
          const element = document.querySelector(root).querySelector(selector);
          if (!element) throw Error('Missing Markdown element: ' + selector);
          const computed = getComputedStyle(element);
          return Object.fromEntries(properties.map(property => [property, computed[property]]));
        });
      }, { selector, properties });
      assert.deepEqual(styles[0], styles[1], selector + ' must match chat styles');
    }
  } finally {
    await browser?.close();
    await server.close();
    await rm(cacheDir, { recursive: true, force: true });
  }
});
