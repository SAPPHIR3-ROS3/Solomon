import assert from 'node:assert/strict';
import { after, test } from 'node:test';
import { existsSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { chromium } from 'playwright-core';

const root = fileURLToPath(new URL('..', import.meta.url));
const server = await createServer({ root, configFile: false, server: { host: '127.0.0.1', port: 0, hmr: false }, esbuild: { jsx: 'automatic' } });
await server.listen();
after(() => server.close());
const executablePath = process.env.SOLOMON_TEST_BROWSER || ['/usr/bin/chromium', '/usr/bin/chromium-browser', '/usr/bin/google-chrome'].find(existsSync);
assert.ok(executablePath, 'Install Chromium or set SOLOMON_TEST_BROWSER');
const browser = await chromium.launch({ executablePath, headless: true, args: ['--no-sandbox'] });
after(() => browser.close());
const { MarkdownContent } = await server.ssrLoadModule('/src/chat/MarkdownContent.tsx');
const markup = renderToStaticMarkup(createElement(MarkdownContent, { content: String.raw`Formula: \(2n+1\).

\[
\boxed{2n\le2n+1\le3n\quad\text{per ogni }n\ge1.}
\]

\(\frac{1}\)` }));

const sourceStyles = ['/node_modules/katex/dist/katex.min.css', '/src/chat/math.css', '/src/chat/chat.css', '/src/chat/chat-timeline.css', '/src/chat/chat-compaction.css'];
// Production CSS has different chunking and order than individual source files.
// Build the GUI before running this test to exercise the actual packaged styles.
const productionIndex = readFileSync(new URL('../dist/index.html', import.meta.url), 'utf8');
const productionStyles = [...productionIndex.matchAll(/href="([^"\s]+\.css)"/g)].map(match => `/dist${match[1]}`);
assert.ok(productionStyles.length > 0);

for (const [mode, styles] of [['source', sourceStyles], ['production', productionStyles]]) {
for (const width of [1100, 390]) {
  test(`math keeps horizontal characters with ${mode} chat styles at ${width}px`, async () => {
    const page = await browser.newPage({ viewport: { width, height: 900 } });
    try {
      await page.route('**/__math_fixture', route => route.fulfill({ contentType: 'text/html', body: `
        ${styles.map(href => `<link rel="stylesheet" href="${href}">`).join('\n')}
        <style>:root { --color-text: #eee; --color-text-muted: #aaa; } body { margin: 8px; } .chat-message { max-width: 100%; }</style>
        <article class="chat-message is-assistant"><span class="fixture-label">Assistant</span>${markup}</article>` }));
      await page.goto(`${server.resolvedUrls.local[0]}__math_fixture`);
      await page.evaluate(() => document.fonts.ready);
      const layout = await page.locator('p .katex').first().evaluate(math => {
        const characters = [...math.querySelectorAll('.katex-html .mord, .katex-html .mbin')];
        return characters.map(character => {
          const style = getComputedStyle(character);
          const rect = character.getBoundingClientRect();
          return { display: style.display, font: style.fontFamily, transform: style.textTransform, margin: style.marginBottom, x: rect.x, y: rect.y, bottom: rect.bottom };
        });
      });
      assert.equal(layout.length, 4);
      for (const character of layout) {
        assert.equal(character.display, 'inline', JSON.stringify(layout));
        assert.ok(character.font.includes('KaTeX'), JSON.stringify(layout));
        assert.equal(character.transform, 'none');
        assert.equal(character.margin, '0px');
      }
      // Different mathematical fonts have different ascenders, but their
      // character boxes must overlap vertically on the same line.
      assert.ok(Math.max(...layout.map(c => c.y)) < Math.min(...layout.map(c => c.bottom)), JSON.stringify(layout));
      assert.ok(layout.at(-1).x > layout[0].x);
      assert.equal(await page.locator('.fixture-label').evaluate(node => getComputedStyle(node).display), 'block');
      assert.equal(await page.locator('.katex-error').evaluate(node => getComputedStyle(node).display), 'inline');
      assert.equal(await page.locator('.katex-display').evaluate(node => getComputedStyle(node).overflowX), 'auto');
    } finally { await page.close(); }
  });
}
}
