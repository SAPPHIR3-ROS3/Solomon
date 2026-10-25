import assert from "node:assert/strict";
import { test, after } from "node:test";
import { existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { chromium } from "playwright-core";

let fixtureModule;
const server = await createServer({ plugins: [{ name: "chat-fixture", resolveId: id => id === "/__chat_fixture.tsx" ? id : null, load: id => id === "/__chat_fixture.tsx" ? fixtureModule : null }], root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { host: "127.0.0.1", port: 0, hmr: false }, esbuild: { jsx: "automatic" } });
await server.listen();
after(() => server.close());
const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/google-chrome"].find(existsSync);
assert.ok(executablePath, "Install Chromium or set SOLOMON_TEST_BROWSER");
const browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
after(() => browser.close());

fixtureModule = `import React, { useState } from "react";
import { createRoot } from "react-dom/client";
import { MarkdownContent } from '/src/chat/MarkdownContent.tsx';
import { MessageFooter, ReasoningBlock } from '/src/chat/ChatMessageFooter.tsx';
import { watchExternalLinks } from '/src/externalLinks.ts';
import { LinkContextMenu } from '/src/shell/LinkContextMenu.tsx';
import '/src/styles.css';
import '/src/chat/chat.css';
window.opened = []; window.copied = [];
window.runtime = { BrowserOpenURL: url => window.opened.push(url), ClipboardSetText: async text => { window.copied.push(text); return true; } };
watchExternalLinks();
const message = { id: 'fixture', role: 'assistant', content: 'Testo della chat con [link](https://example.com/doc).', reasoning: 'Ragionamento selezionabile senza chiuderlo.' };
function Fixture() {
  const [collapsed, setCollapsed] = useState(false);
  return React.createElement('div', { className: 'app-shell' }, React.createElement('div', { className: 'chat-view', style: { padding: '60px', width: '900px' } },
    React.createElement('article', { className: 'chat-message is-assistant' },
      React.createElement(ReasoningBlock, { message, isCollapsed: collapsed, onToggle: () => setCollapsed(value => !value) }),
      React.createElement(MarkdownContent, { content: message.content }),
      React.createElement(MessageFooter, { message, index: 0 })),
    React.createElement('article', { className: 'chat-message is-user', style: { width: '100%' } }, React.createElement(MarkdownContent, { content: ['Prima riga mobile ', 'seconda riga mobile', '', 'Nuovo paragrafo  ', 'interruzione esplicita.'].join(String.fromCharCode(10)) })),
    React.createElement('textarea', { 'aria-label': 'Message input' }),
    React.createElement(LinkContextMenu)));
}
createRoot(document.getElementById('root')).render(React.createElement(Fixture));
`;
const fixture = '<!doctype html><html><body><div id="root"></div><script type="module" src="/__chat_fixture.tsx"></script></body></html>';

test("chat selection, external links, message copy and paste", async () => {
  const page = await browser.newPage({ viewport: { width: 1000, height: 600 } });
  page.setDefaultTimeout(10000);
  page.on('pageerror', error => process.stderr.write(`${error.message}\n`));
  try {
    await page.route("**/__chat_fixture", route => route.fulfill({ contentType: "text/html", body: fixture }));
    await page.goto(`${server.resolvedUrls.local[0]}__chat_fixture`);
    const reasoning = page.locator('.chat-reasoning-copy');
    await reasoning.waitFor();
    assert.equal(await reasoning.evaluate(node => getComputedStyle(node).userSelect), 'text');
    assert.equal(await page.locator('.chat-message.is-assistant p').evaluate(node => getComputedStyle(node).userSelect), 'text');
    const box = await reasoning.boundingBox();
    await page.mouse.move(box.x + 1, box.y + box.height / 2);
    await page.mouse.down();
    await page.mouse.move(box.x + box.width - 1, box.y + box.height / 2, { steps: 15 });
    await page.mouse.up();
    assert.match(await page.evaluate(() => window.getSelection().toString()), /Ragionamento/);
    assert.equal(await page.locator('.chat-reasoning').getAttribute('aria-expanded'), 'true');
    await page.getByRole('link', { name: 'link', exact: true }).click();
    assert.deepEqual(await page.evaluate(() => window.opened), ['https://example.com/doc']);
    assert.ok(page.url().endsWith('/__chat_fixture'));
    const link = page.getByRole('link', { name: 'link', exact: true });
    await link.click({ button: 'right' });
    assert.deepEqual(await page.evaluate(() => window.opened), ['https://example.com/doc']);
    await page.getByRole('menuitem', { name: 'Copy link address', exact: true }).click();
    assert.deepEqual(await page.evaluate(() => window.copied), ['https://example.com/doc']);
    await link.click({ button: 'right' });
    await page.getByRole('menuitem', { name: 'Open in browser', exact: true }).click();
    assert.deepEqual(await page.evaluate(() => window.opened), ['https://example.com/doc', 'https://example.com/doc']);
    await link.focus();
    await page.keyboard.press('Shift+F10');
    await page.getByRole('menu', { name: 'Link actions' }).waitFor();
    await page.keyboard.press('ArrowDown');
    assert.equal(await page.getByRole('menuitem', { name: 'Copy link address' }).evaluate(node => node === document.activeElement), true);
    await page.keyboard.press('Escape');
    assert.equal(await page.getByRole('menu').count(), 0);
    assert.equal(await link.evaluate(node => node === document.activeElement), true);
    await link.click({ button: 'right' });
    await page.getByRole('textbox', { name: 'Message input' }).click();
    assert.equal(await page.getByRole('menu').count(), 0);
    await page.evaluate(() => { window.copied = []; delete window.runtime; });
    await link.dispatchEvent('contextmenu');
    assert.equal(await page.getByRole('menu').count(), 0);
    await page.evaluate(() => { window.runtime = { BrowserOpenURL: url => window.opened.push(url), ClipboardSetText: async text => { window.copied.push(text); return true; } }; });
    await page.getByRole('button', { name: 'Copy message', exact: true }).click();
    await page.getByRole('button', { name: 'Message copied', exact: true }).waitFor();
    assert.deepEqual(await page.evaluate(() => window.copied), ['Testo della chat con [link](https://example.com/doc).']);
    // The browser clipboard is isolated from the desktop clipboard. Native
    // Wails calls above are stubbed; this checks the standard paste shortcut.
    await page.context().grantPermissions(['clipboard-read', 'clipboard-write']);
    await page.evaluate(() => navigator.clipboard.writeText(window.copied[0]));
    await page.getByRole('textbox', { name: 'Message input' }).focus();
    await page.keyboard.press('Control+V');
    assert.equal(await page.getByRole('textbox', { name: 'Message input' }).inputValue(), 'Testo della chat con [link](https://example.com/doc).');
  } finally { await page.close(); }
});


test("mobile user text reflows after resizing while preserving paragraphs and explicit breaks", async () => {
  const page = await browser.newPage({ viewport: { width: 1000, height: 900 } });
  try {
    await page.route("**/__chat_fixture", route => route.fulfill({ contentType: "text/html", body: fixture }));
    await page.goto(`${server.resolvedUrls.local[0]}__chat_fixture`);
    const paragraphs = page.locator('.chat-message.is-user p');
    await paragraphs.first().waitFor();
    assert.equal(await paragraphs.count(), 2);
    const lineCount = () => paragraphs.first().evaluate(node => {
      const range = document.createRange(); range.selectNodeContents(node);
      return new Set(Array.from(range.getClientRects(), rect => Math.round(rect.top))).size;
    });
    assert.equal(await lineCount(), 1);
    assert.equal(await paragraphs.nth(1).locator('br').count(), 1);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.locator('.chat-view').evaluate(node => { node.style.width = '240px'; node.style.padding = '10px'; });
    assert.ok(await lineCount() > 1);
    await page.locator('.chat-view').evaluate(node => { node.style.width = '900px'; });
    await page.setViewportSize({ width: 1000, height: 900 });
    assert.equal(await lineCount(), 1);
  } finally { await page.close(); }
});
