import assert from 'node:assert/strict';
import { after, test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { existsSync } from 'node:fs';
import { createServer } from 'vite';
import { chromium } from 'playwright-core';
import react from '@vitejs/plugin-react';

const fixtureModule = `
import React, { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { AtMentionInput } from '/src/home/AtMentionInput.tsx';
window.nativeImage = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aD1sAAAAASUVORK5CYII=';
window.nativeDelay = 0;
const clips = [];
window.go = { main: { ServerBridge: { URL: async () => location.origin, ClipboardImage: () => new Promise(resolve => setTimeout(() => resolve(window.nativeImage), window.nativeDelay)) } } };
function Fixture() {
  const [value, setValue] = useState('');
  const [images, setImages] = useState([]);
  return <><AtMentionInput aria-label="Message" value={value} onChange={setValue} clips={clips} images={images} onImagesChange={setImages} /><output>{images.length}</output></>;
}
createRoot(document.getElementById('root')).render(<Fixture />);`;
const server = await createServer({ root: fileURLToPath(new URL('..', import.meta.url)), configFile: false, plugins: [react(), { name: 'paste-fixture', resolveId: id => id === '/__paste.tsx' ? id : null, load: id => id === '/__paste.tsx' ? fixtureModule : null }], server: { host: '127.0.0.1', port: 0, hmr: false } });
await server.listen();
after(() => server.close());
const browser = await chromium.launch({ executablePath: process.env.SOLOMON_TEST_BROWSER || ['/usr/bin/chromium', '/usr/bin/google-chrome'].find(existsSync), headless: true, args: ['--no-sandbox'] });
after(() => browser.close());

test('composer pastes browser images once, recovers native images, and preserves text paste', async () => {
  const page = await browser.newPage();
  page.setDefaultTimeout(10000);
  page.on('pageerror', error => process.stderr.write(`${error.message}\n`));
  try {
    await page.route('**/__paste', route => route.fulfill({ contentType: 'text/html', body: '<div id="root"></div><script type="module" src="/__paste.tsx"></script>' }));
    await page.goto(`${server.resolvedUrls.local[0]}__paste`);
    const input = page.getByRole('textbox', { name: 'Message', exact: true });
    await input.focus();
    // A WebView may never emit a paste event for an image. Only the shortcut fires.
    await input.evaluate(node => node.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, key: 'v', ctrlKey: true })));
    await page.waitForFunction(() => document.querySelector('output').textContent === '1');
    assert.equal((await input.inputValue()).trim(), '[img-0]');

    // A browser exposing the image must not also attach the native fallback.
    await page.evaluate(async () => {
      window.nativeDelay = 100;
      const node = document.querySelector('textarea');
      const blob = await (await fetch(window.nativeImage)).blob();
      node.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, key: 'v', ctrlKey: true }));
      const clipboardData = new DataTransfer();
      clipboardData.items.add(new File([blob], 'screenshot.png', { type: 'image/png' }));
      node.dispatchEvent(new ClipboardEvent('paste', { bubbles: true, cancelable: true, clipboardData }));
    });
    await page.waitForFunction(() => document.querySelector('output').textContent === '2');
    await page.waitForTimeout(150);
    assert.equal(await page.locator('output').textContent(), '2');
    assert.equal((await input.inputValue()).match(/\[img-1\]/g).length, 1);

    // A genuine text paste retains browser behavior, even with a pending read.
    await page.context().grantPermissions(['clipboard-read', 'clipboard-write']);
    await page.evaluate(() => navigator.clipboard.writeText('testo incollato'));
    await page.keyboard.press('Control+V');
    await page.waitForFunction(() => document.querySelector('textarea').value.includes('testo incollato'));
    await page.waitForTimeout(150);
    assert.equal(await page.locator('output').textContent(), '2');

    // Edits made while the native read is pending invalidate that read.
    await input.evaluate(node => node.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, key: 'v', ctrlKey: true })));
    await input.fill('nuovo messaggio');
    await page.waitForTimeout(150);
    assert.equal(await input.inputValue(), 'nuovo messaggio');
    assert.equal(await page.locator('output').textContent(), '0');

    // Context-menu paste can supply an empty event instead of a shortcut.
    await input.evaluate(node => node.dispatchEvent(new ClipboardEvent('paste', { bubbles: true, cancelable: true, clipboardData: new DataTransfer() })));
    await page.waitForFunction(() => document.querySelector('output').textContent === '1');
    assert.ok((await input.inputValue()).includes('[img-2]'));
  } finally { await page.close(); }
});
