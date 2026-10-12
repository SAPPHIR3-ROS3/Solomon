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
