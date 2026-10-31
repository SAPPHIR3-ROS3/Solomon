import assert from 'node:assert/strict';
import { after, test } from 'node:test';
import { createServer } from 'vite';
const server = await createServer({ configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { keepImagePreviewOpen } = await server.ssrLoadModule('/src/chat/imagePreview.ts');
test('contained image hit testing respects letterboxing, padding, upscaling and unloaded images', () => {
  for (const [w,h,x,y,expected] of [[2000,600,516,316,true],[2000,600,516,50,false],[2000,600,516,600,false],[300,600,516,316,true],[300,600,50,316,false],[300,600,1000,316,false],[100,60,20,20,true],[100,60,10,10,false],[0,0,516,316,false]]) {
    let stopped = false;
    keepImagePreviewOpen({currentTarget:{naturalWidth:w,naturalHeight:h,getBoundingClientRect:()=>({left:16,top:16,width:1000,height:600})},clientX:x,clientY:y,stopPropagation(){stopped=true;}});
    assert.equal(stopped,expected,JSON.stringify([w,h,x,y]));
  }
});
