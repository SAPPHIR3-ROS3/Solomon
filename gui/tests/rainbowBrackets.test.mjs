import assert from 'node:assert/strict';
import { after, test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const root = fileURLToPath(new URL('..', import.meta.url));
const server = await createServer({ root, configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { rainbowBrackets } = await server.ssrLoadModule('/src/syntax/rainbowBrackets.ts');
const { cursorTokenizer } = await server.ssrLoadModule('/src/syntax/cursorSyntax.ts');
const yellow = '#ffd700', fuchsia = '#ff00ff', sky = '#87cefa';
const colored = lines => lines.flat().filter(token => [yellow, fuchsia, sky].includes(token.color)).map(token => [token.content, token.color]);

test('cycles nesting colors across tokens and lines, pairing all three bracket types', () => {
  const source = ['({', '[()]', '})'];
  const tokens = source.map(content => [...content].map((content, offset) => ({ content, offset })));
  const result = rainbowBrackets(tokens);
  assert.deepEqual(colored(result), [
    ['(', yellow], ['{', fuchsia], ['[', sky], ['(', yellow],
    [')', yellow], [']', sky], ['}', fuchsia], [')', yellow],
  ]);
  assert.deepEqual(result.map(line => line.map(token => token.content).join('')), source);
});

test('uses grammar token types to ignore strings, comments and regular expressions', async () => {
  const source = 'call(\n  "[", /* } */ /[()]/, // )\n  [{ value: (1) }]\n)';
  const { highlighter, language } = await cursorTokenizer('example.ts');
  const tokens = highlighter.codeToTokens(source, { lang: language, theme: 'cursor-dark', includeExplanation: 'tokenType' }).tokens;
  const result = rainbowBrackets(tokens);
  assert.deepEqual(colored(result), [
    ['(', yellow], ['[', fuchsia], ['{', sky], ['(', yellow],
    [')', yellow], ['}', sky], [']', fuchsia], [')', yellow],
  ]);
  assert.equal(result.map(line => line.map(token => token.content).join('')).join('\n'), source);
  for (const token of result.flat()) assert.equal(source.slice(token.offset, token.offset + token.content.length), token.content);
});

test('keeps unmatched closers from corrupting depth and recalculates after edits', () => {
  const render = content => colored(rainbowBrackets([[{ content, offset: 0 }]]));
  assert.deepEqual(render('(])'), [['(', yellow], [')', yellow]]);
  assert.deepEqual(render('([{}])'), [['(', yellow], ['[', fuchsia], ['{', sky], ['}', sky], [']', fuchsia], [')', yellow]]);
  assert.deepEqual(render('[{}]'), [['[', yellow], ['{', fuchsia], ['}', fuchsia], [']', yellow]]);
});
