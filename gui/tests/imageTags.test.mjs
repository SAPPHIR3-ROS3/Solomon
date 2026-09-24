import assert from 'node:assert/strict';
import { after, test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const root = fileURLToPath(new URL('..', import.meta.url));

const server = await createServer({ root, configFile: false, server: { middlewareMode: true, hmr: false }, esbuild: { jsx: 'automatic' } });
after(() => server.close());
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
const { MarkdownContent } = await server.ssrLoadModule('/src/chat/MarkdownContent.tsx');
const tag = (id) => '[' + 'img-' + id + ']';
const render = (content) => renderToStaticMarkup(createElement(MarkdownContent, { content }));
const badges = (html) => (html.match(/class="composer-image-tag chat-image-tag"/g) ?? []).length;

test('sent and reopened content renders identical badges without metadata', () => {
  const content = 'Before ' + tag(0) + ' ' + tag(12) + ' ' + tag(0) + ' after';
  const saved = JSON.parse(JSON.stringify({ content }));
  const html = render(content);
  assert.equal(badges(html), 3);
  assert.equal(render(saved.content), html);
  assert.ok(html.includes('>' + tag(12) + '</span>'));
  assert.equal(saved.content, content);
});

test('preserves emphasis, lists and GFM tables', () => {
  const html = render('**' + tag(1) + '**\n\n- ' + tag(2) + '\n\n| Image |\n| --- |\n| ' + tag(3) + ' |');
  assert.equal(badges(html), 3);
  for (const element of ['strong', 'li', 'table', 'td']) assert.ok(html.includes('<' + element));
});

test('does not decorate inline, fenced or indented code', () => {
  const tick = String.fromCharCode(96);
  const html = render(tick + tag(1) + tick + '\n\n' + tick.repeat(3) + 'text\n' + tag(2) + '\n' + tick.repeat(3) + '\n\n    ' + tag(3));
  assert.equal(badges(html), 0);
  assert.ok(html.includes('<code>' + tag(1) + '</code>'));
  assert.ok(html.includes('chat-code-block'));
});

test('preserves links, reference links and image alt text', () => {
  const html = render('[' + tag(1) + '](./file.png) [' + tag(2) + '][ref] ![' + tag(3) + '](./image.png)\n\n[ref]: https://example.com');
  assert.equal(badges(html), 0);
  assert.ok(html.includes('href="./file.png"'));
  assert.ok(html.includes('chat-file-link'));
  assert.ok(html.includes('alt="' + tag(3) + '"'));
  assert.ok(html.includes('href="https://example.com"'));
});

test('leaves malformed tags and ordinary Markdown unchanged', () => {
  assert.equal(badges(render(tag('x') + tag(-1) + tag('') + '**bold**')), 0);
  assert.equal(render('Plain **bold**'), '<p>Plain <strong>bold</strong></p>');
});

test('does not enable raw HTML execution', () => {
  const html = render('<script>alert(1)</script>\n\n' + tag(0));
  assert.ok(!html.includes('<script>'));
  assert.equal(badges(html), 1);
});
test('keeps a lone image tag that Markdown would treat as a reference link', () => {
  const html = render(tag(0));
  assert.equal(badges(html), 1);
  assert.ok(html.includes('>' + tag(0) + '</span>'));
});

test('decorates terminal selection tags', () => {
  const tag = '[terminal-L3-L7]';
  const html = render('Output ' + tag + ' after');
  assert.equal((html.match(/chat-terminal-tag/g) ?? []).length, 1);
  assert.ok(html.includes('>' + tag + '</span>'));
});
