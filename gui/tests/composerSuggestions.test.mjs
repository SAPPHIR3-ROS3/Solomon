import assert from 'node:assert/strict';
import { after, test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const server = await createServer({ root: fileURLToPath(new URL('..', import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { suggestionContext, insertSuggestion } = await server.ssrLoadModule('/src/home/composerSuggestions.ts');

test('offers slash commands only at the start of a message', () => {
  assert.equal(suggestionContext('/', 1).kind, 'command');
  assert.equal(suggestionContext('  /he', 5).query, 'he');
  for (const value of ['Explain /help', '/docs query', 'https://example.com', '/dir/file']) {
    assert.equal(suggestionContext(value, value.length), null);
  }
});

test('offers files in messages and command arguments, but not email addresses', () => {
  for (const value of ['@', 'Read @src/main.go', '/docs @readme']) {
    assert.equal(suggestionContext(value, value.length).kind, 'file');
  }
  assert.equal(suggestionContext('user@example.com', 16), null);
  assert.equal(suggestionContext('Read @file next', 15), null);
});

test('replaces the entire token when completing in its middle', () => {
  const value = 'Read @old/file then explain';
  const result = insertSuggestion(value, suggestionContext(value, 9), '@new.go');
  assert.equal(result.value, 'Read @new.go then explain');
  assert.equal(result.cursor, 13);
  assert.deepEqual(insertSuggestion('/he', suggestionContext('/he', 3), '/help'), { value: '/help ', cursor: 6 });
});
