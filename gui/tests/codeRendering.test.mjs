import assert from 'node:assert/strict';
import { after, test } from 'node:test';
import { readFile } from 'node:fs/promises';
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

test('keeps adjacent inline code in summary lists without inventing code blocks', () => {
  const content = '1. header intent\n2. `Tool: orchestrate`\n3. **`SDK calls N · 2.12s`**, niente label Execution\n4. `Source`\n5. `Result`\n\n`Source · Go` ridotto a solo `Source`.';
  const html = render(content);
  assert.equal((html.match(/class="chat-code-block"/g) ?? []).length, 0);
  assert.ok(html.includes('<strong><code>SDK calls N · 2.12s</code></strong>'));
  assert.ok(html.includes('<li><code>Source</code></li>'));
  assert.ok(html.includes('<li><code>Result</code></li>'));
});

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
  assert.ok(html.includes('class="chat-line-indent">            </span><span class="chat-line-body">memo[cell] = paths(row - 1, col) + paths(row, col - 1)</span>'));
  assert.ok(html.includes('<p>Il risultato è <code>2</code>.</p>'));
  assert.equal(render(JSON.parse(JSON.stringify({ content })).content), html);
});

test('keeps an unlabeled ASCII tree as text, including a collapsed single-backtick span', () => {
  const tree = ['        5', '      /   \\', '     2     7', '    / \\   / \\', '   1   3 6   8'].join('\n');
  for (const wrapped of [tick.repeat(3) + '\n' + tree + '\n' + tick.repeat(3), tick + '\n' + tree + '\n' + tick]) {
    const html = render('Prima.\n\n' + wrapped + '\n\nDopo.');
    assert.equal(html.includes('chat-tree'), false);
    assert.equal((html.match(/class="chat-code-line"/g) ?? []).length, 5);
    for (const line of tree.split('\n')) {
      const indent = line.match(/^[ \t]*/)?.[0] ?? '';
      assert.ok(html.includes('class="chat-line-indent">' + indent + '</span><span class="chat-line-body">' + line.slice(indent.length) + '</span>'));
    }
    assert.ok(html.includes('<p>Dopo.</p>'));
  }
});

test('preserves inline code, unknown labels and literal backticks', () => {
  assert.equal(render(tick + 'python' + tick), '<p><code>python</code></p>');
  for (const content of [tick + 'unknown\nexample\n' + tick, tick + 'python\nprint("' + tick + '")\n' + tick]) {
    assert.ok(!render(content).includes('chat-code-block'));
  }
  const literal = tick.repeat(3) + 'python\n' + tick + 'python\nexample\n' + tick + '\n' + tick.repeat(3);
  const html = render(literal);
  assert.equal((html.match(/class="chat-code-block"/g) ?? []).length, 1);
  assert.ok(html.includes('>' + tick + 'python</span>'));
});


test('code snippets wrap long lines inside the block', async () => {
  const css = await readFile(new URL('../src/chat/chat.css', import.meta.url), 'utf8');
  assert.match(css, /\.chat-code-block pre \{[^}]*overflow-x: hidden;/);
  assert.match(css, /\.chat-code-block pre code \{[^}]*white-space: pre-wrap;/);
  assert.match(css, /\.chat-code-block \{[^}]*tab-size: 2;/);
  assert.match(css, /\.chat-line-content \{[^}]*min-width: 0;[^}]*white-space: pre-wrap;/);
  assert.match(css, /\.chat-line-indent \{[^}]*white-space: pre;/);
  assert.match(css, /\.chat-line-body \{[^}]*min-width: 0;[^}]*white-space: pre-wrap;/);
  const indented = render(tick.repeat(3) + 'python\n' + 'def f():\n\treturn value\n  keep\n' + tick.repeat(3));
  assert.ok(indented.includes('class="chat-line-indent">\t</span>'));
  assert.ok(indented.includes('class="chat-line-indent">  </span>'));
  assert.ok(indented.includes('>return value</span>'));
});

test('user messages retain the exact input whitespace and indentation', () => {
  const content = 'mentre ci sono ancora elementi in level:\n  distance+1\n  per ogni elemento e in level:\n    se e non è in visited:\n      nextlevel += g[e]\n      visited += [e]\n  level = next level\n  nextlevel = set vuoto';
  const renderUser = (text) => renderToStaticMarkup(createElement(MarkdownContent, { content: text, preserveLineBreaks: true }));
  assert.equal(renderUser(content), '<div class="chat-user-text">' + content + '</div>');
  assert.equal(renderUser('prima\n\n  seconda\n\tterza  riga'), '<div class="chat-user-text">prima\n\n  seconda\n\tterza  riga</div>');
  assert.equal(renderUser('**literal** <tag>'), '<div class="chat-user-text">**literal** &lt;tag&gt;</div>');
  assert.equal(render('prima\n**seconda**'), '<p>prima\n<strong>seconda</strong></p>');
});
