import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import assert from 'node:assert/strict';
import { after, test } from 'node:test';
import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';
import { chromium } from 'playwright-core';

const fixture = `
import {useRef,useState} from 'react';
import {createRoot} from 'react-dom/client';
import {AtMentionInput} from '/src/home/AtMentionInput.tsx';
import {FloatingLetters} from '/src/chat/FloatingLetters.tsx';
import '/src/home/welcome.css';
function Fixture(){
 const viewRef=useRef(null);
 const [value,setValue]=useState('[img-16] [img-17] [img-18]');
 const [images,setImages]=useState([16,17,18].map(id=>({id,tag:'[img-'+id+']',name:id+'.png',url:'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"/>'})));
 const [content,setContent]=useState('');
 window.updateResponse=setContent;
 return <section className="chat-view" ref={viewRef} style={{position:'relative',height:600,width:800}}>
 <FloatingLetters content={content} messageID="answer" working={true} viewRef={viewRef}/>
 <div className="chat-messages-shell" style={{height:350}}><div className="chat-response-content" data-message-id="answer">{content}</div></div>
 <div className="chat-composer-dock"><div className="chat-composer-background"/><AtMentionInput aria-label="Message" className="welcome-input" value={value} onChange={setValue} images={images} onImagesChange={setImages}/></div>
 </section>;
}
createRoot(document.getElementById('root')).render(<Fixture/>);
`;
const testCacheDir = mkdtempSync(join(tmpdir(), "solomon-vite-test-"));
const server = await createServer({ cacheDir: testCacheDir,root:fileURLToPath(new URL('..',import.meta.url)),configFile:false,esbuild:{jsx:'automatic'},server:{host:'127.0.0.1',port:0,hmr:false},plugins:[{
 name:'composer-image-regression',resolveId:id=>id==='/__images.tsx'?id:undefined,load:id=>id==='/__images.tsx'?fixture:undefined,
}]});
await server.listen();
after(async () => { await server.close(); rmSync(testCacheDir, { recursive: true, force: true }); });
const browser=await chromium.launch({executablePath:process.env.SOLOMON_TEST_BROWSER||['/usr/bin/chromium','/usr/bin/google-chrome'].find(existsSync),headless:true,args:['--no-sandbox']});
after(()=>browser.close());

test('adjacent image overlays match literal text spacing and deleting previews lands existing flights',async()=>{
 const page=await browser.newPage();
 const errors=[];
 page.on('pageerror',error=>errors.push(error.message));
 await page.route('**/__solomon/**',route=>route.fulfill({json:{commands:[],suggestions:[]}}));
 await page.route('**/__images',route=>route.fulfill({contentType:'text/html',body:'<style>.welcome-input{width:100%!important}</style><div id="root" class="app-shell" data-client-surface="desktop" data-client-os="linux"></div><script type="module" src="/__images.tsx"></script>'}));
 await page.goto(server.resolvedUrls.local[0]+'__images');
 await page.waitForFunction(()=>document.querySelectorAll('.composer-image-tag').length===3);
 assert.deepEqual(errors,[]);
 assert.equal(await page.locator('.chat-composer-background').evaluate(element=>getComputedStyle(element).backdropFilter),'none');
 const gaps=await page.evaluate(()=>{
  const tags=[...document.querySelectorAll('.composer-image-tag')].map(tag=>tag.getBoundingClientRect());
  return tags.slice(1).map((tag,index)=>tag.left-tags[index].right);
 });
 assert.ok(Math.abs(gaps[0]-gaps[1])<0.5,`unequal tag spacing: ${gaps}`);
 await page.evaluate(()=>window.updateResponse('Letters arriving'));
 await page.waitForFunction(()=>[...CSS.highlights.values()].some(value=>value.size>0));
 await page.getByRole('button',{name:'Remove 16.png',exact:true}).dispatchEvent('mousedown');
 await page.getByRole('button',{name:'Remove 17.png',exact:true}).dispatchEvent('mousedown');
 await page.getByRole('button',{name:'Remove 18.png',exact:true}).dispatchEvent('mousedown');
 await page.waitForFunction(()=>!document.querySelector('.composer-image-previews'));
 await page.waitForFunction(()=>[...CSS.highlights.values()].every(value=>value.size===0),null,{timeout:150});
 assert.equal(await page.locator('.chat-response-content').textContent(),'Letters arriving');
 await page.evaluate(()=>window.updateResponse('Letters arriving again'));
 await page.waitForFunction(()=>[...CSS.highlights.values()].some(value=>value.size>0));
 assert.deepEqual(errors,[]);
 await page.close();
});
