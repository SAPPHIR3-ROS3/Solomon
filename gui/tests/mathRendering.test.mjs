import assert from 'node:assert/strict';
import { after, test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const root = fileURLToPath(new URL('..', import.meta.url));
const server = await createServer({ root, configFile: false, server: { middlewareMode: true, hmr: false }, esbuild: { jsx: 'automatic' } });
after(() => server.close());
const { MarkdownContent } = await server.ssrLoadModule('/src/chat/MarkdownContent.tsx');
const render = (content) => renderToStaticMarkup(createElement(MarkdownContent, { content }));
const formulas = (html) => (html.match(/class="katex"/g) ?? []).length;

test('renders the TeX notation, display equations and table from reference thread 8b1e8247', () => {
  const content = String.raw`Sì: **\(O\) maiuscola e \(o\) minuscola hanno significati diversi**.

\[
\lim_{n\to\infty}\frac{f(n)}{g(n)}=0.
\]

\[
\boxed{2n\le2n+1\le3n\quad\text{per ogni }n\ge1.}
\]

| Notazione | Idea |
|---|---|
| \(O(g(n))\) | Cresce **al massimo come** \(g(n)\) |
| \(o(g(n))\) | Cresce **strettamente più lentamente** |
| \(\Theta(g(n))\) | Cresce **dello stesso ordine** |`;
  const html = render(content);
  assert.equal(formulas(html), 8);
  assert.equal((html.match(/class="katex-display"/g) ?? []).length, 2);
  assert.ok(html.includes('<math xmlns="http://www.w3.org/1998/Math/MathML"'));
  assert.match(html, /<table\b/);
  assert.ok(!html.includes('katex-error'));
  assert.ok(!html.includes('chat-code-block'));
  assert.equal(render(JSON.parse(JSON.stringify({ content })).content), html);
});

test('supports dollar delimiters and math fences', () => {
  const html = render('$x^2$\n\n$$\n\\frac{1}{n}\n$$\n\n```math\nx+y\n```');
  assert.equal(formulas(html), 3);
  assert.equal((html.match(/class="katex-display"/g) ?? []).length, 2);
  assert.ok(!html.includes('chat-code-block'));
});

test('preserves TeX examples in inline, fenced, indented and unclosed code', () => {
  for (const code of [
    '`\\(x\\)`',
    '``\\(x\\) ` example``',
    '```tex\n\\[x\\]\n```',
    '~~~~text\n\\(x\\)\n~~~~',
    '    \\(x\\)',
    '```tex\n\\(x\\)',
  ]) {
    const html = render(code);
    assert.equal(formulas(html), 0, code);
    assert.ok(html.includes('\\(') || html.includes('\\['), code);
  }
});

test('keeps escaped delimiters and incomplete streaming formulas as text', () => {
  for (const content of [String.raw`\\(x\\)`, String.raw`before \(x`, String.raw`before \[x`]) {
    assert.equal(formulas(render(content)), 0);
  }
  assert.equal(formulas(render(String.raw`before \(x\) after`)), 1);
});

test('invalid and untrusted TeX does not crash rendering or create active links', () => {
  assert.ok(render(String.raw`\(\frac{1}\)`).includes('katex-error'));
  const html = render(String.raw`\(\href{javascript:alert(1)}{x}\)`);
  assert.ok(!html.includes('href="javascript:'));
  assert.ok(!html.includes('<script'));
});

test('math rendering ignores macro definitions inherited from Object.prototype', () => {
  const macro = String.raw`\dependabotPollutedMacro`;
  const content = String.raw`\(\dependabotPollutedMacro\)`;
  const previous = Object.getOwnPropertyDescriptor(Object.prototype, macro);
  try {
    Object.defineProperty(Object.prototype, macro, { configurable: true, value: 'POLLUTED' });
    const html = render(content);
    // Unknown commands may render as error text or a katex-error fallback.
    assert.match(html, /(?:<mtext>|class="katex-error"[^>]*>)\\dependabotPollutedMacro/);
    assert.ok(!html.includes('<mi>P</mi>'));
    assert.equal(formulas(render(String.raw`\(x^2\)`)), 1);
  } finally {
    if (previous) Object.defineProperty(Object.prototype, macro, previous);
    else delete Object.prototype[macro];
  }
});

test('preserves Markdown and image badges beside formulas', () => {
  const html = render(String.raw`- **Formula:** \(n_0=1\) [img-0]
- [file](./file.md)`);
  assert.equal(formulas(html), 1);
  assert.ok(html.includes('<strong>Formula:</strong>'));
  assert.ok(html.includes('chat-image-tag'));
  assert.ok(html.includes('chat-file-link'));
});
