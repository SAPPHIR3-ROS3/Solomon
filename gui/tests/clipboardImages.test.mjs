import assert from 'node:assert/strict';
import { after, test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const server = await createServer({ root: fileURLToPath(new URL('..', import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
const { clipboardImageFiles, readNativeClipboardImage } = await server.ssrLoadModule('/src/home/clipboardImages.ts');
after(async () => { delete globalThis.window; await server.close(); });

test('accepts image items and falls back to files without duplicating images', () => {
  const image = new File(['pixels'], 'image.png', { type: 'image/png' });
  const text = new File(['text'], 'text.txt', { type: 'text/plain' });
  assert.deepEqual(clipboardImageFiles({ items: [], files: [image, text] }), [image]);
  assert.deepEqual(clipboardImageFiles({ items: [{ kind: 'file', type: 'image/png', getAsFile: () => image }], files: [image] }), [image]);
  assert.deepEqual(clipboardImageFiles({ items: [{ kind: 'file', type: 'image/png', getAsFile: () => null }], files: [image] }), [image]);
});

test('converts native image bytes into an attachment and preserves empty clipboard behavior', async () => {
  globalThis.window = {};
  assert.deepEqual(await readNativeClipboardImage(), []);
  window.go = { main: { ServerBridge: { ClipboardImage: async () => '' } } };
  assert.deepEqual(await readNativeClipboardImage(), []);
  window.go.main.ServerBridge.ClipboardImage = async () => 'data:image/png;base64,AQID';
  const [image] = await readNativeClipboardImage();
  assert.equal(image.type, 'image/png');
  assert.equal(image.name, 'clipboard.png');
  assert.deepEqual([...new Uint8Array(await image.arrayBuffer())], [1, 2, 3]);
});
