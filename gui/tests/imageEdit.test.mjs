import assert from 'node:assert/strict';
import { test } from 'node:test';
import { createServer } from 'vite';

const root = new URL('..', import.meta.url).pathname;

const server = await createServer({ root, configFile: false, server: { middlewareMode: true, hmr: false } });
const mod = await server.ssrLoadModule('/src/home/imageEdit.ts');
await server.close();

test('maps pointer position onto canvas pixels', () => {
  const point = mod.canvasPoint({ left: 10, top: 20, width: 100, height: 50 }, 60, 45, 200, 80);
  assert.deepEqual(point, { x: 100, y: 40 });
});

test('moves and scales strokes without mutating the original', () => {
  const stroke = { color: '#fff', kind: 'rectangle', points: [{ x: 10, y: 10 }, { x: 30, y: 20 }], size: 8 };
  const moved = mod.moveStroke(stroke, 5, -2);
  assert.deepEqual(moved.points[0], { x: 15, y: 8 });
  assert.deepEqual(stroke.points[0], { x: 10, y: 10 });
  const scaled = mod.scaleStroke(stroke, { x: 10, y: 10 }, 2);
  assert.deepEqual(scaled.points[1], { x: 50, y: 30 });
});

test('cropping shifts strokes into the new origin', () => {
  const cropped = mod.cropStrokes([{ color: '#fff', kind: 'freehand', points: [{ x: 20, y: 30 }], size: 4 }], { x: 10, y: 10 }, { x: 40, y: 50 });
  assert.deepEqual(cropped.box, { left: 10, top: 10, width: 30, height: 40 });
  assert.deepEqual(cropped.strokes[0].points[0], { x: 10, y: 20 });
});

test('hit testing uses padded stroke bounds', () => {
  const stroke = { color: '#fff', kind: 'text', points: [{ x: 10, y: 10 }], size: 10, text: 'Hi' };
  assert.equal(mod.strokeContains(stroke, { x: 12, y: 12 }), true);
  assert.equal(mod.strokeContains(stroke, { x: 200, y: 200 }), false);
});
