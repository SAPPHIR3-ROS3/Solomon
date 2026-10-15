import assert from "node:assert/strict";
import { after, test } from "node:test";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";

const server = await createServer({root:fileURLToPath(new URL('..',import.meta.url)),configFile:false,server:{middlewareMode:true,hmr:false}});
after(()=>server.close());
const { reconcileFloatingText } = await server.ssrLoadModule('/src/chat/floatingLetterDiff.ts');

for (const [before, afterText] of [
  ['**Settled letters','Settled letters'],
  ['Before `existing code','Before existing code'],
  ['[Existing label](https://example.com','Existing label'],
  ['**already** and *settled','already and settled'],
  ['same text','same text'],
]) {
  test(`formatting retains existing letters: ${before}`,()=>{
    const changes = reconcileFloatingText(before,afterText);
    assert.deepEqual(changes.addedOffsets,[]);
    const retained=[];
    let offset=0;
    for(const glyph of before){
      const next=changes.retainedOffset(offset);
      if(next!==undefined){ assert.equal(afterText.slice(next,next+glyph.length),glyph); retained.push(next); }
      offset+=glyph.length;
    }
    assert.equal(retained.length,Array.from(afterText).length);
  });
}

test('only fresh suffix letters animate when formatting changes at the same time',()=>{
  const changes = reconcileFloatingText('**settled','settled new');
  assert.deepEqual(changes.addedOffsets,[7,8,9,10]);
  assert.equal(changes.retainedOffset(2),0);
});

test('repeated letters and Unicode retain distinct positions',()=>{
  const before='**caffe\u0301 🤌🏽 one one';
  const afterText='caffe\u0301 🤌🏽 one one plus';
  const changes=reconcileFloatingText(before,afterText);
  assert.equal(changes.retainedOffset(before.indexOf('🤌')),afterText.indexOf('🤌'));
  assert.equal(changes.addedOffsets.map(offset=>String.fromCodePoint(afterText.codePointAt(offset))).join(''),' plus');
});

test('large rewrites preserve matching letters without a quadratic table',()=>{
  const word='abcdefghijk'.repeat(1000);
  const changes=reconcileFloatingText('**'+word,word+'!');
  assert.deepEqual(changes.addedOffsets,[word.length]);
  assert.equal(changes.retainedOffset(2),0);
  assert.equal(changes.retainedOffset(word.length+1),word.length-1);
});
