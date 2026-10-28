import assert from 'node:assert/strict';
import { after, test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const server = await createServer({ root: fileURLToPath(new URL('..', import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
const { clipboardImageFiles, isImageFileDrag, nativeImageDropPaths, readNativeDroppedImages, readNativeClipboardImage } = await server.ssrLoadModule('/src/home/clipboardImages.ts');
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

test('detects image drags before protected file bytes are available', () => {
  for (const type of ['image/png', 'image/jpeg', 'image/webp', '']) {
    assert.equal(isImageFileDrag({items: [{kind: 'file', type, getAsFile: () => null}], files: [], types: ['Files']}), true);
  }
  assert.equal(isImageFileDrag({items: [], files: [], types: ['Files']}), true);
  assert.equal(isImageFileDrag({items: [], files: [new File(['pixels'], 'photo.webp', {type: 'image/webp'})]}), true);
  assert.equal(isImageFileDrag({items: [{kind: 'string', type: 'text/plain'}], files: [], types: ['text/plain']}), false);
  assert.equal(isImageFileDrag({items: [{kind: 'file', type: 'text/plain'}], files: [], types: ['Files']}), false);
  assert.equal(isImageFileDrag({items: [], files: [], types: []}), false);
});

test('accepts desktop image files with missing MIME types without accepting other files', async () => {
  for (const [name, type] of [['Screenshot.PNG', 'image/png'], ['photo.jpg', 'image/jpeg'], ['photo.jpeg', 'image/jpeg'], ['picture.webp', 'image/webp']]) {
    const file = new File(['image bytes'], name);
    const [image] = clipboardImageFiles({items: [{kind: 'file', type: '', getAsFile: () => file}], files: [file]});
    assert.equal(image.name, name);
    assert.equal(image.type, type);
    assert.equal(await image.text(), 'image bytes');
  }
  assert.deepEqual(clipboardImageFiles({items: [], files: [new File(['text'], 'notes.txt')]}), []);
});

test('recognizes native URI and multiline path drops without fetching remote URLs', () => {
  const transfer = (uri, text = '') => ({getData: type => type === 'text/uri-list' ? uri : text});
  assert.deepEqual(nativeImageDropPaths(transfer('# comment\r\nfile:///home/oni/Screen%201.png\r\nfile:///home/oni/Screen%232.jpeg\r\nfile:///home/oni/Screen3.webp')), ['/home/oni/Screen 1.png', '/home/oni/Screen#2.jpeg', '/home/oni/Screen3.webp']);
  assert.deepEqual(nativeImageDropPaths(transfer('', '/home/oni/Screen 1.png\n"/home/oni/Screen 2.jpeg"\n/home/oni/Screen 3.webp')), ['/home/oni/Screen 1.png', '/home/oni/Screen 2.jpeg', '/home/oni/Screen 3.webp']);
  assert.deepEqual(nativeImageDropPaths(transfer('file:///C:/Pictures/image.png')), ['C:/Pictures/image.png']);
  for (const value of ['https://example.com/image.png', 'file://remote-host/image.png', 'file:///home/oni/notes.txt']) assert.deepEqual(nativeImageDropPaths(transfer(value)), []);
  assert.deepEqual(nativeImageDropPaths(transfer('', 'ordinary text')), []);
});

test('native path drops become real File attachments with their original names', async () => {
  let received;
  globalThis.window = {go: {app: {ServerBridge: {ReadDroppedImages: async paths => {received = paths; return paths.map((name, i) => ({name: name.split('/').at(-1), data: `data:image/png;base64,${btoa(String.fromCharCode(i + 1))}`}));}}}}};
  const paths = ['/home/oni/one.png', '/home/oni/two.png', '/home/oni/three.png'];
  const images = await readNativeDroppedImages(paths);
  assert.deepEqual(received, paths);
  assert.deepEqual(images.map(image => image.name), ['one.png', 'two.png', 'three.png']);
  assert.deepEqual(images.map(image => image.type), ['image/png', 'image/png', 'image/png']);
  assert.deepEqual(await Promise.all(images.map(async image => [...new Uint8Array(await image.arrayBuffer())])), [[1], [2], [3]]);
});
