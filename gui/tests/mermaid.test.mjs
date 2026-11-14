import assert from 'node:assert/strict';
import { test } from 'node:test';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';
import { chromium } from 'playwright-core';
import { launchDesktopWebKit } from '../scripts/webkit-driver.mjs';

test('renders diagrams and recovers from invalid streaming source', async () => {
  const fixture = `import React from 'react';
    import {createRoot} from 'react-dom/client';
    import {MarkdownContent} from '/src/chat/MarkdownContent.tsx';
    import '/src/theme/themes.css';
    window.runtime={ClipboardSetText:async text=>{window.copied=text;return true;}};
    const root=createRoot(document.getElementById('root'));
    window.show=content=>root.render(<MarkdownContent content={content}/>);`;
  const cacheDir = await mkdtemp(join(tmpdir(), 'solomon-mermaid-'));
  const server = await createServer({
    cacheDir,
    root: fileURLToPath(new URL('..', import.meta.url)), configFile: false,
    optimizeDeps: { noDiscovery: true, include: ['react', 'react-dom/client', 'react-dom', 'react/jsx-runtime', 'react/jsx-dev-runtime', 'react-markdown', 'remark-gfm', 'remark-math', 'rehype-katex', 'katex', '@shikijs/core', '@shikijs/engine-oniguruma', '@shikijs/engine-oniguruma/wasm-inlined', 'mermaid'] },
    esbuild: { jsx: 'automatic' }, server: { host: '127.0.0.1', port: 0, hmr: false },
    plugins: [{
      name: 'mermaid-fixture',
      resolveId: id => id === '/fixture.tsx' ? id : undefined,
      load: id => id === '/fixture.tsx' ? fixture : undefined,
      configureServer(server) {
        server.middlewares.use((req, res, next) => {
          if (req.url !== '/fixture') return next();
          res.setHeader('Content-Type', 'text/html');
          res.end('<!doctype html><div id="root"></div><script>window.errors=[];window.addEventListener("error",e=>window.errors.push(e.message));window.addEventListener("unhandledrejection",e=>window.errors.push(String(e.reason)));</script><script type="module" src="/fixture.tsx"></script>');
        });
      },
    }],
  });
  let browser;
  try {
    await server.listen();
    browser = process.env.SOLOMON_TEST_ENGINE === 'webkitgtk' ? await launchDesktopWebKit() : await chromium.launch({ executablePath: process.env.SOLOMON_TEST_BROWSER || '/usr/bin/chromium', headless: true, args: ['--no-sandbox'] });
    const page = await browser.newPage();
    await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`);
    await page.waitForFunction(() => typeof window.show === 'function' || window.errors?.length);
    assert.deepEqual(await page.evaluate(() => window.errors), []);
    const show = code => page.evaluate(content => window.show(content), '```mermaid\n' + code + '\n```');
    await show('flowchart TD\nA[Il tool termina] --> B[Ricevi output]\nB --> C{Output comprimibile?}');
    await page.waitForFunction(() => document.querySelector('.chat-mermaid-preview img')?.naturalWidth > 0 || document.querySelector('.chat-mermaid-status')?.title);
    assert.equal(await page.evaluate(() => document.querySelector('.chat-mermaid-status')?.title || ''), '');
    assert.match(decodeURIComponent(await page.evaluate(() => document.querySelector('.chat-mermaid-preview img').getAttribute('src'))), /Ricevi/);
    assert.equal(await page.evaluate(() => document.querySelector('pre')), null);
    await page.evaluate(() => document.querySelector('button[aria-label="Copy SVG"]').click());
    await page.waitForFunction(() => window.copied?.startsWith('<svg'));
    assert.match(await page.evaluate(() => window.copied), /#0d3566/);
    assert.match(await page.evaluate(() => window.copied), /#3b8fd1/);
    await page.evaluate(() => [...document.querySelectorAll('.chat-mermaid-switch button')].find(button => button.textContent === 'Source').click());
    await page.waitForFunction(() => !!document.querySelector('pre'));
    assert.equal(await page.evaluate(() => document.querySelector('.chat-mermaid-preview')), null);
    await page.evaluate(() => document.querySelector('button[aria-label="Copy text"]').click());
    await page.waitForFunction(() => window.copied?.startsWith('flowchart TD'));
    assert.match(await page.evaluate(() => window.copied), /Output comprimibile/);
    await page.evaluate(() => document.querySelector('.chat-mermaid-switch button').click());
    await show('not a valid diagram');
    await page.waitForFunction(() => document.querySelector('.chat-mermaid-status')?.textContent.includes('Preview unavailable'));
    assert.equal(await page.evaluate(() => document.querySelector('button[aria-label="Copy SVG"]').disabled), true);
    await page.evaluate(() => [...document.querySelectorAll('.chat-mermaid-switch button')].find(button => button.textContent === 'Source').click());
    await page.waitForFunction(() => !!document.querySelector('pre'));
    assert.match(await page.evaluate(() => document.querySelector('pre').innerText), /not a valid diagram/);
    await page.evaluate(() => document.querySelector('.chat-mermaid-switch button').click());
    await show('sequenceDiagram\nAlice->>Bob: Hello');
    await page.waitForFunction(() => document.querySelector('.chat-mermaid-preview img')?.naturalWidth > 0 || document.querySelector('.chat-mermaid-status')?.title);
    assert.equal(await page.evaluate(() => document.querySelector('.chat-mermaid-status')?.title || ''), '');
    assert.match(decodeURIComponent(await page.evaluate(() => document.querySelector('.chat-mermaid-preview img').getAttribute('src'))), /Hello/);
    await page.evaluate(() => window.show('```js\nconst x = 1;\n```'));
    await page.waitForFunction(() => !document.querySelector('.chat-mermaid-preview'));
  } finally {
    await browser?.close();
    await server.close();
    await rm(cacheDir, { recursive: true, force: true });
  }
});
