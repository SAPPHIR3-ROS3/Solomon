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
const tick = String.fromCharCode(96);

test('recovers dotenv and PowerShell blocks from thread 45b61937', () => {
  const env = 'OPENMEDIAGEN_BIND_HOST=127.0.0.1\nOPENMEDIAGEN_PORT=8000\n\nOPENMEDIAGEN_MODELS_DIR=./models\nOPENMEDIAGEN_OUTPUT_DIR=./output\n\nOPENMEDIAGEN_API_KEY=\nOPENMEDIAGEN_HF_TOKEN=';
  for (const [language, code] of [['dotenv', env], ['powershell', 'docker compose up -d --build']]) {
    const malformed = 'Configurazione:\n' + tick + language + '\n' + code + '\n' + tick;
    const fenced = 'Configurazione:\n' + tick.repeat(3) + language + '\n' + code + '\n' + tick.repeat(3);
    const html = render(malformed);
    assert.equal(html, render(fenced));
    assert.equal((html.match(/class="chat-code-block"/g) ?? []).length, 1);
    assert.equal((html.match(/class="chat-code-line"/g) ?? []).length, code.split('\n').length);
    assert.ok(html.includes('aria-label="Copy code"'));
  }
});

test('recovers the recursive Python snippet from thread 125a0e35 with its indentation', () => {
  const code = [
    'def count_paths(n, blocked):',
    '    memo = {}',
    '',
    '    def paths(row, col):',
    '        if row < 0 or col < 0 or (row, col) == blocked:',
    '            return 0',
    '        if row == 0 and col == 0:',
    '            return 1',
    '',
    '        cell = (row, col)',
    '        if cell not in memo:',
    '            memo[cell] = paths(row - 1, col) + paths(row, col - 1)',
    '',
    '        return memo[cell]',
    '',
    '    return paths(n - 1, n - 1)',
    '',
    '',
    'print(count_paths(3, (1, 1)))  # 2',
  ].join('\n');
  const content = 'Versione ricorsiva:\n\n' + tick + 'python\n' + code + '\n' + tick + '\n\nIl risultato è ' + tick + '2' + tick + '.';
  const html = render(content);
  assert.equal(html, render(content.replace(tick + 'python\n', tick.repeat(3) + 'python\n').replace('\n' + tick + '\n', '\n' + tick.repeat(3) + '\n')));
  assert.equal((html.match(/class="chat-code-line"/g) ?? []).length, 19);
  assert.ok(html.includes('            memo[cell] = paths(row - 1, col) + paths(row, col - 1)'));
  assert.ok(html.includes('<p>Il risultato è <code>2</code>.</p>'));
  assert.equal(render(JSON.parse(JSON.stringify({ content })).content), html);
});

test('preserves inline code, unknown labels, incomplete blocks and literal backticks', () => {
  assert.equal(render(tick + 'python' + tick), '<p><code>python</code></p>');
  for (const content of [tick + 'unknown\nexample\n' + tick, tick + 'python\nunclosed', tick + 'python\nprint("' + tick + '")\n' + tick]) {
    assert.ok(!render(content).includes('chat-code-block'));
  }
  const literal = tick.repeat(3) + 'python\n' + tick + 'python\nexample\n' + tick + '\n' + tick.repeat(3);
  const html = render(literal);
  assert.equal((html.match(/class="chat-code-block"/g) ?? []).length, 1);
  assert.ok(html.includes('>' + tick + 'python</span>'));
});


test('GUI user messages preserve soft line breaks without changing assistant rendering or code', () => {
  const content = 'prima riga\nseconda **riga**\nterza riga';
  const renderUser = (text) => renderToStaticMarkup(createElement(MarkdownContent, { content: text, preserveLineBreaks: true }));
  const standard = render(content);
  assert.equal(standard, '<p>prima riga\nseconda <strong>riga</strong>\nterza riga</p>');
  assert.equal(renderUser(content), '<p>prima riga<br/>\nseconda <strong>riga</strong><br/>\nterza riga</p>');
  assert.equal(render(content), standard, 'cached user rendering must not affect assistant messages');
  assert.equal(renderUser('prima\n\nseconda'), '<p>prima</p>\n<p>seconda</p>');
  assert.equal(renderUser('prima  \nseconda'), '<p>prima<br/>\nseconda</p>');
  const code = tick.repeat(3) + 'text\nprima\nseconda\n' + tick.repeat(3);
  assert.equal(renderUser(code), render(code), 'code blocks must remain literal');
  assert.equal(renderUser(tick + 'prima\nseconda' + tick), render(tick + 'prima\nseconda' + tick));
});
