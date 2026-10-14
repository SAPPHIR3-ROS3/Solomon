import assert from "node:assert/strict";
import { after, test } from "node:test";
import { existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import react from "@vitejs/plugin-react";
import { chromium } from "playwright-core";

const source = 'package main\nfunc main() { println("hello", 42) }';
const pythonSource = 'raise ValueError("k must be between 1 and len(values)")';
const markdown = '```typescript\nconst answer = 42; // comment\n```\n\n```python\n' + pythonSource + '\n```';
const fixture = `import React from 'react';
import {createRoot} from 'react-dom/client';
import {MarkdownContent} from '/src/chat/MarkdownContent.tsx';
import {ToolCallCard} from '/src/chat/ChatToolActivity.tsx';
import '/src/styles.css'; import '/src/chat/chat.css'; import '/src/theme/themes.css';
window.copied = [];
Object.defineProperty(navigator, 'clipboard', {value: {writeText: async text => {window.copied.push(text)}}});
createRoot(document.getElementById('root')).render(React.createElement('article',{className:'chat-message is-assistant'},
React.createElement(MarkdownContent,{content:${JSON.stringify(markdown)}}),
React.createElement(ToolCallCard,{tool:{id:'source',name:'orchestrate',intent:'Show source',input:${JSON.stringify(source)},status:'success',defaultOpen:true,result:{status:'success',output:'done'}}})));
`;
const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, plugins: [react(), {
  name: "cursor-syntax-fixture",
  resolveId: id => id === "/__cursor_fixture.jsx" ? id : null,
  load: id => id === "/__cursor_fixture.jsx" ? fixture : null,
}], server: { host: "127.0.0.1", port: 0, hmr: false } });
await server.listen();
after(() => server.close());
const executablePath = ["/usr/bin/chromium", "/usr/bin/google-chrome"].find(existsSync);
const browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
after(() => browser.close());

test("markdown and CODE snippets use Cursor tokens and copy the original source", async () => {
  const page = await browser.newPage({viewport:{width:1100,height:900}});
  after(() => page.close());
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  page.on('console', message => {if(message.type()==='error' && message.text().includes('highlight'))errors.push(message.text())});
  await page.route('**/__cursor_fixture', route => route.fulfill({contentType:'text/html',body:'<div id="root" class="app-shell"></div><script type="module" src="/__cursor_fixture.jsx"></script>'}));
  await page.route('**/__solomon/gui-settings', route => route.fulfill({json:{chat:{startToolCallsCollapsed:false,autoCloseToolCalls:false,startReasoningCollapsed:false}}}));
  await page.route('**/__solomon/format-go', route => route.fulfill({json:{source}}));
  await page.goto(`${server.resolvedUrls.local[0]}__cursor_fixture`);
  await page.waitForFunction(() => [...document.querySelectorAll('.chat-line-content span')].some(node => node.textContent==='const' && getComputedStyle(node).color==='rgb(130, 210, 206)'));
  await page.getByRole('button',{name:'Copy code',exact:true}).first().click();
  assert.deepEqual(await page.evaluate(()=>window.copied),['const answer = 42; // comment\n']);
  await page.waitForFunction(() => [...document.querySelectorAll('.chat-line-content span')].some(node => node.textContent === 'raise'));
  const snippets = await page.locator('.chat-code-block').evaluateAll(blocks => blocks.map(block => {
    const line = block.querySelector('.chat-line-content');
    const tokens = [...line.querySelectorAll('span')];
    return {text: line.textContent, numbers: [...block.querySelectorAll('.chat-line-number')].map(node => node.textContent), tokens: tokens.map(node => {
      const style = getComputedStyle(node);
      return {display: style.display, transform: style.textTransform, fontSize: style.fontSize, fontFamily: style.fontFamily, marginBottom: style.marginBottom, top: node.getBoundingClientRect().top};
    })};
  }));
  assert.equal(snippets[1].text, pythonSource);
  for (const snippet of snippets) {
    assert.deepEqual(snippet.numbers, ['1']);
    for (const token of snippet.tokens) {
      assert.equal(token.display, 'inline');
      assert.equal(token.transform, 'none');
      assert.equal(token.fontSize, '12px');
      assert.match(token.fontFamily, /Geist Mono/);
      assert.equal(token.marginBottom, '0px');
    }
    assert.ok(Math.max(...snippet.tokens.map(token => token.top)) - Math.min(...snippet.tokens.map(token => token.top)) < 2, 'Tokens of a source line must remain on one visual line');
  }
  await page.getByRole('button',{name:'Copy code',exact:true}).nth(1).click();
  assert.deepEqual(await page.evaluate(()=>window.copied), ['const answer = 42; // comment\n', pythonSource + '\n']);
  await page.getByRole('button',{name:'Show source',exact:true}).click();
  await page.waitForFunction(() => [...document.querySelectorAll('.chat-code-mode-source .cm-cursor-token')].some(node => node.textContent==='func' && getComputedStyle(node).color==='rgb(130, 210, 206)'));
  assert.deepEqual(errors,[]);
});
