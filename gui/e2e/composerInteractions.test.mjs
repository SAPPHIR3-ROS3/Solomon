import assert from 'node:assert/strict';
import { after, afterEach, test } from 'node:test';
import { existsSync } from 'node:fs';
import { mkdtemp, rm } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';
import { chromium } from 'playwright-core';
import { launchDesktopWebKit } from '../scripts/webkit-driver.mjs';

const fixture = `
import React from 'react';
import {createRoot} from 'react-dom/client';
import {ChatView} from '/src/chat/ChatView.tsx';
import {applyLiveStreamEvent} from '/src/chat/chatClient.ts';
import '/src/styles.css';
import '/src/home/welcome.css';
window.interactionErrors=[];
window.addEventListener('error',event=>window.interactionErrors.push(event.message));
window.addEventListener('unhandledrejection',event=>window.interactionErrors.push(String(event.reason)));
const initial = {id:'composer-interactions',projectID:'fixture',title:'Interactions',status:'done',messages:[
 {id:'u0',role:'user',content:'Earlier question'},
 {id:'a0',role:'assistant',content:'Earlier response',checkpointSeq:1,workedFor:14,workStartedAt:1000},
]};
function Fixture(){
 const [chat,setChat]=React.useState(initial);
 window.setTranscript=(messages,status='done')=>setChat({...initial,status,messages});
 window.streamReasoning=delta=>setChat(current=>applyLiveStreamEvent(current,{type:'assistant_delta',channel:'reasoning',delta}));
 window.startAssistant=()=>setChat(current=>applyLiveStreamEvent(current,{type:'assistant_start',turn:2,checkpoint_seq:2}));
 window.setComposer=(text,cursor=text.length)=>{
  const input=document.querySelector('textarea[aria-label="Message"]');
  input.focus();
  Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(input,text);
  input.dispatchEvent(new Event('input',{bubbles:true}));
  input.setSelectionRange(cursor,cursor);
  input.dispatchEvent(new MouseEvent('click',{bubbles:true}));
 };
 window.submitComposer=()=>document.querySelector('textarea[aria-label="Message"]').dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',code:'Enter',bubbles:true,cancelable:true}));
 return <ChatView chat={chat} isStreaming={chat.status==='running'} onSend={(_,message)=>setChat(current=>({...current,status:'running',runStartedAt:Date.now(),messages:[...current.messages,message]}))} onDeleteMessage={()=>{}} onStopTool={()=>{}} onStopStreaming={()=>{}}/>;
}
createRoot(document.getElementById('root')).render(<Fixture/>);
`;
const cacheDir=await mkdtemp(join(tmpdir(),'solomon-composer-tests-'));
after(()=>rm(cacheDir,{recursive:true,force:true}));
const server=await createServer({root:fileURLToPath(new URL('..',import.meta.url)),cacheDir,configFile:false,esbuild:{jsx:'automatic'},server:{host:'127.0.0.1',port:0,hmr:false},plugins:[{
 name:'composer-interactions',resolveId:id=>id==='/__interactions.tsx'?id:undefined,load:id=>id==='/__interactions.tsx'?fixture:undefined,
 configureServer(server){server.middlewares.use((req,res,next)=>{
  if(req.url==='/__interactions'){res.setHeader('Content-Type','text/html');res.end('<!doctype html><div id="root" class="app-shell"></div><script type="module" src="/__interactions.tsx"></script>');return;}
  if(req.url?.startsWith('/__solomon/')){res.setHeader('Content-Type','application/json');res.end(JSON.stringify({chat:{floatingLetters:false},projects:[],commands:[],suggestions:[],providers:[],recent:[],current:{provider:'',model:''}}));return;}next();
 });}
}]});
await server.listen();after(()=>server.close());
const engine=process.env.SOLOMON_TEST_ENGINE || 'chromium';
const browser=engine==='webkitgtk'?await launchDesktopWebKit():await chromium.launch({executablePath:process.env.SOLOMON_TEST_BROWSER || ['/usr/bin/chromium','/usr/bin/google-chrome'].find(existsSync),headless:true,args:['--no-sandbox']});
after(()=>browser.close());
const page=await browser.newPage();page.setDefaultTimeout(10000);
afterEach(async()=>assert.deepEqual(await page.evaluate(()=>window.interactionErrors??[]),[]));
const load=async()=>{await page.goto(server.resolvedUrls.local[0]+'__interactions');await page.waitForFunction(()=>!!document.querySelector('textarea'));};

test('sending a new user turn does not start a timer above it',async()=>{
 await load();
 await page.evaluate(()=>window.setComposer('Next question'));
 await page.evaluate(()=>window.submitComposer());
 await page.waitForFunction(()=>[...document.querySelectorAll('.is-user')].some(node=>node.textContent.includes('Next question')));
 assert.equal(await page.evaluate(()=>document.querySelectorAll('.chat-worked-for.is-live').length),0);
 assert.equal(await page.evaluate(()=>document.querySelector('.chat-worked-for')?.textContent),'worked for 14s');
 await page.waitForTimeout(600);
 assert.equal(await page.evaluate(()=>document.querySelectorAll('.chat-worked-for.is-live').length),0);
 await page.evaluate(()=>window.startAssistant());
 await page.waitForFunction(()=>!!document.querySelector('.chat-worked-for.is-live'));
 assert.ok(await page.evaluate(()=>{
  const user=[...document.querySelectorAll('.is-user')].at(-1);
  return Boolean(user.compareDocumentPosition(document.querySelector('.chat-worked-for.is-live')) & Node.DOCUMENT_POSITION_FOLLOWING);
 }));
});

test('composer wraps complete words and keeps the caret on the rendered word',async()=>{
 await load();
 // The word fits on its own but not on the remainder of the first line.
 const sample=await page.evaluate(()=>{
  const input=document.querySelector('textarea');
  const style=getComputedStyle(input);
  const width=input.clientWidth-parseFloat(style.paddingLeft)-parseFloat(style.paddingRight);
  const ctx=document.createElement('canvas').getContext('2d');ctx.font=style.font;
  const word='interfaccia';
  let prefix='Una ';
  while(ctx.measureText(prefix+'test '+word.slice(0,-1)).width < width) prefix+='test ';
  // Set the available width to a boundary between the penultimate and final character.
  const target=ctx.measureText(prefix+word.slice(0,-1)).width+ctx.measureText('a').width/2;
  const composer=document.querySelector('.welcome-composer');
  composer.style.width=(target+parseFloat(getComputedStyle(composer).paddingLeft)+parseFloat(getComputedStyle(composer).paddingRight)+2+(input.getBoundingClientRect().width-input.clientWidth))+'px';
  return {prefix,word};
 });
 const text=sample.prefix+sample.word;
 await page.evaluate(text=>window.setComposer(text),text);
 await page.waitForTimeout(150);
 // Measure the complete text, without splitting it into spans at the caret:
 // a zero-width marker or a prefix-only mirror can introduce a false wrap.
 const measure=async (cursor,value=text)=>{
  await page.evaluate(({text,cursor})=>window.setComposer(text,cursor),{text:value,cursor});
  await page.waitForTimeout(100);
  return page.evaluate(({text,cursor})=>{
   const input=document.querySelector('textarea'), shell=input.parentElement, style=getComputedStyle(input);
   const mirror=document.createElement('div');
   for(const property of ['boxSizing','font','letterSpacing','lineHeight','padding','whiteSpace','wordBreak','overflowWrap','wordSpacing','tabSize'])mirror.style[property]=style[property];
   mirror.style.position='fixed';mirror.style.width=input.clientWidth+'px';mirror.style.visibility='hidden';
   mirror.textContent=text;document.body.append(mirror);
   const wordStart=text.lastIndexOf(' ')+1;
   const range=document.createRange();range.setStart(mirror.firstChild,wordStart);range.setEnd(mirror.firstChild,text.length);
   const rects=[...range.getClientRects()];
   const caretRange=document.createRange();caretRange.setStart(mirror.firstChild,cursor);caretRange.collapse(true);
   const expected=caretRange.getBoundingClientRect(), bounds=mirror.getBoundingClientRect();
   const caret=document.querySelector('.composer-image-caret').getBoundingClientRect(), parent=shell.getBoundingClientRect();
   const data={wordLines:rects.length,wordTop:rects[0].top-bounds.top,expectedLeft:expected.left-bounds.left+input.getBoundingClientRect().left-parent.left,expectedTop:expected.top-bounds.top+input.getBoundingClientRect().top-parent.top,caretLeft:caret.left-parent.left,caretTop:caret.top-parent.top,scrollLeft:input.scrollLeft,scrollWidth:input.scrollWidth,clientWidth:input.clientWidth};
   mirror.remove();return data;
  },{text:value,cursor});
 };
 const shorter=text.slice(0,-1);
 const before=await measure(shorter.length,shorter);
 const end=await measure(text.length);
 assert.ok(before.wordTop<10,'the incomplete word must initially fit on the first line');
 assert.equal(end.wordLines,1,'a word that fits must move to the next line as a whole');
 assert.ok(end.wordTop>10,'the word must wrap before the right padding');
 assert.equal(end.scrollLeft,0);
 const middle=await measure(sample.prefix.length+3);
 assert.ok(Math.abs(middle.caretTop-middle.expectedTop)<4,'caret must follow the entire wrapped word when editing its middle');
 assert.ok(Math.abs(middle.caretLeft-middle.expectedLeft)<3,'caret must not remain at the end of the previous line');
 const deleted=await measure(shorter.length,shorter);
 assert.ok(deleted.wordTop<10,'deleting the last character must bring the complete word back');
 const multiline='Prima riga\nSeconda riga\n';
 await page.evaluate(text=>window.setComposer(text),multiline);
 await page.waitForTimeout(100);
 assert.equal(await page.evaluate(()=>document.querySelector('textarea').value),multiline);
 const long='abcdefghij'.repeat(100);
 await page.evaluate(text=>window.setComposer(text),long);
 await page.waitForTimeout(100);
 assert.ok(await page.evaluate(()=>{const input=document.querySelector('textarea');return input.scrollWidth<=input.clientWidth+1 && input.scrollLeft===0;}),'words longer than the entire composer must still fit without horizontal scrolling');
 // Changing the layout width must update height and overlays without another keystroke.
 await page.evaluate(()=>{document.querySelector('.welcome-composer').style.width='350px';});
 await page.waitForTimeout(150);
 assert.ok(await page.evaluate(()=>{const input=document.querySelector('textarea'),caret=document.querySelector('.composer-image-caret');return input.scrollWidth<=input.clientWidth+1 && parseFloat(caret.style.left)<input.clientWidth;}));

});

test('reasoning is available exactly when its text exists, including streamed and collapsed tool activity',async()=>{
 await load();
 for(const [index,reasoning] of [undefined,'',' \n\t'].entries()){
  await page.evaluate(({index,reasoning})=>window.setTranscript([{id:'empty-'+index,role:'assistant',content:'Answer without reasoning',reasoning,thoughtFor:4,stats:{ttftSeconds:4}}]),{index,reasoning});
  await page.waitForFunction(index=>!!document.querySelector('[data-message-id="empty-'+index+'"]'),index);
  assert.equal(await page.evaluate(()=>document.querySelector('.chat-reasoning-brief')?.textContent),'thought briefly');
  await page.evaluate(()=>{
   const label=document.querySelector('.chat-reasoning-brief');
   label.click();
   label.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true}));
  });
  assert.ok(await page.evaluate(()=>{
   const label=document.querySelector('.chat-reasoning-brief');
   return !label.hasAttribute('role') && !label.hasAttribute('tabindex') && !label.hasAttribute('aria-expanded') && !document.querySelector('.chat-reasoning-copy');
  }),'the brief label must not be an expandable control');
 }
 for(const [index,thoughtFor] of [undefined,3].entries()){
  await page.evaluate(({index,thoughtFor})=>window.setTranscript([{id:'real-'+index,role:'assistant',content:'Answer with reasoning',reasoning:'Available reasoning text',thoughtFor}]),{index,thoughtFor});
  await page.waitForFunction(()=>document.querySelector('.chat-reasoning-copy')?.textContent==='Available reasoning text');
  await page.evaluate(()=>document.querySelector('.chat-reasoning').click());
  await page.waitForFunction(()=>document.querySelector('.chat-reasoning')?.getAttribute('aria-expanded')==='false');
  assert.equal(await page.evaluate(()=>document.querySelectorAll('.chat-reasoning-copy').length),0);
  await page.evaluate(()=>document.querySelector('.chat-reasoning').click());
  await page.waitForFunction(()=>document.querySelector('.chat-reasoning-copy')?.textContent==='Available reasoning text');
 }
 await page.evaluate(()=>window.setTranscript([{id:'streaming-reasoning',role:'assistant',content:'Streaming answer',thoughtFor:4}],'running'));
 await page.waitForFunction(()=>!!document.querySelector('[data-message-id="streaming-reasoning"]'));
 assert.equal(await page.evaluate(()=>document.querySelector('.chat-reasoning-brief')?.textContent),'thought briefly');
 await page.evaluate(()=>window.streamReasoning('Reasoning received during streaming'));
 await page.waitForFunction(()=>document.querySelector('.chat-reasoning-copy')?.textContent==='Reasoning received during streaming');
 for(const available of [false,true]){
  await page.evaluate(available=>{
   const tool=id=>({id,name:'shell',input:'echo test',status:'success',result:{status:'success',output:'done'}});
   window.setTranscript([
    {id:'tool-empty-'+available,role:'assistant',content:'',thoughtFor:4,toolCalls:[tool('t1')]},
    {id:'tool-real-'+available,role:'assistant',content:'',reasoning:available?'Actual tool reasoning':undefined,thoughtFor:2,toolCalls:[tool('t2')]},
    {id:'tool-answer-'+available,role:'assistant',content:'Final answer',thoughtFor:6},
   ]);
  },available);
  await page.waitForFunction(available=>!!document.querySelector('[data-message-id="tool-answer-'+available+'"]'),available);
  await page.evaluate(()=>document.querySelector('.chat-tool-collapse-all').click());
  await page.waitForFunction(()=>document.querySelector('.chat-tool-collapse-all')?.getAttribute('aria-expanded')==='false');
  assert.equal(await page.evaluate(()=>document.querySelectorAll('.chat-reasoning-summary').length),available?1:0);
  if(!available) assert.equal(await page.evaluate(()=>document.querySelector('.chat-tool-collapsed-summary .chat-reasoning-brief')?.textContent),'thought briefly');
  if(available) assert.equal(await page.evaluate(()=>document.querySelector('.chat-reasoning-summary').textContent),'thought for 2s','aggregate only durations belonging to available reasoning');
  await page.evaluate(()=>document.querySelector('.chat-tool-collapse-all').click());
  await page.waitForFunction(()=>document.querySelector('.chat-tool-collapse-all')?.getAttribute('aria-expanded')==='true');
  assert.equal(await page.evaluate(()=>document.querySelector('.chat-reasoning-copy')?.textContent??''),available?'Actual tool reasoning':'');
 }
});
