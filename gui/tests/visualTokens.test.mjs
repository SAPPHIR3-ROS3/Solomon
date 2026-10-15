import assert from "node:assert/strict";
import { after, test } from "node:test";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: {middlewareMode:true,hmr:false} });
after(() => server.close());
const { nextVisualTokenEnd } = await server.ssrLoadModule('/src/chat/usePacedChatMessages.ts');

test("visual tokens preserve Unicode graphemes, whitespace and the complete response", () => {
  const source = "Caffe\u0301 👨‍👩‍👧‍👦 🤌🏽 risposta\nseconda riga";
  const segments = new Intl.Segmenter(undefined,{granularity:'grapheme'});
  const boundaries = new Set([...segments.segment(source)].map(part => part.index + part.segment.length));
  let start = 0;
  const tokens = [];
  while (start < source.length) {
    const end = nextVisualTokenEnd(source,start);
    assert.ok(end > start && boundaries.has(end));
    const token = source.slice(start,end);
    assert.ok([...segments.segment(token)].length<=4);
    tokens.push(token);
    start = end;
  }
  assert.equal(tokens.join(''),source);
  assert.ok(tokens.length>5);
});
