import assert from 'node:assert/strict';
import { after, afterEach, test } from 'node:test';
import { existsSync } from 'node:fs';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
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
import {ChatComposer} from '/src/chat/ChatComposer.tsx';
import {applyLiveStreamEvent} from '/src/chat/chatClient.ts';
import {watchImageDrops} from '/src/home/imageDrops.ts';
import '/src/styles.css';
import '/src/home/welcome.css';
window.interactionErrors=[];
window.addEventListener('error',event=>window.interactionErrors.push(event.message));
window.addEventListener('unhandledrejection',event=>window.interactionErrors.push(String(event.reason)));
let stopDropWatch=watchImageDrops();
window.restartDropWatch=()=>{stopDropWatch();stopDropWatch=watchImageDrops();};
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
 return <ChatView chat={chat} isStreaming={chat.status==='running'} onSend={(_,message)=>{window.lastSentMessage=message;setChat(current=>({...current,status:'running',runStartedAt:Date.now(),messages:[...current.messages,message]}));}} onDeleteMessage={()=>{}} onStopTool={()=>{}} onStopStreaming={()=>{}}/>;
}
const root=createRoot(document.getElementById('root'));
window.hideComposer=()=>root.render(<div>No active composer</div>);
window.showKeptAliveComposers=()=>root.render(<>
 <div id="inactive-composer" style={{visibility:'hidden'}} aria-hidden="true"><ChatComposer aria-label="Inactive" draftKey={"inactive-drop-test-"+performance.timeOrigin} /></div>
 <div id="active-composer"><ChatComposer aria-label="Active" draftKey={"active-drop-test-"+performance.timeOrigin} /></div>
</>);
root.render(<Fixture/>);
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

const input = {
 focus: () => page.evaluate(()=>document.querySelector('textarea').focus()),
 inputValue: () => page.evaluate(()=>document.querySelector('textarea').value),
 evaluate: fn => page.evaluate(fn=>{
  return (0,eval)('('+fn+')')(document.querySelector('textarea'));
 },fn.toString()),
};
const keyboard=page.keyboard ?? {
 type: text=>page.evaluate(text=>document.execCommand('insertText',false,text),text),
 press: shortcut=>page.evaluate(shortcut=>{
  const parts=shortcut.split('+');
  document.querySelector('textarea').dispatchEvent(new KeyboardEvent('keydown',{key:parts.at(-1),ctrlKey:parts.includes('Control'),shiftKey:parts.includes('Shift'),bubbles:true,cancelable:true}));
 },shortcut),
};
test('text undo and redo preserve selection and discard stale redo', async()=>{
 await load();
 await input.focus();
 await keyboard.type('Hello');
 await keyboard.press('Control+z');
 assert.equal(await input.inputValue(),'');
 await keyboard.press('Control+Shift+z');
 assert.equal(await input.inputValue(),'Hello');
 await page.evaluate(()=>document.querySelector('textarea').setSelectionRange(1,4));
 await keyboard.type('i');
 assert.equal(await input.inputValue(),'Hio');
 await keyboard.press('Control+z');
 assert.equal(await input.inputValue(),'Hello');
 assert.deepEqual(await input.evaluate(input=>[input.selectionStart,input.selectionEnd]),[1,4]);
 await keyboard.press('Control+y');
 assert.equal(await input.inputValue(),'Hio');
 await keyboard.press('Control+z');
 await keyboard.type('ey');
 const changed=await input.inputValue();
 await keyboard.press('Control+y');
 assert.equal(await input.inputValue(),changed);
});

test('clearing after send resets text history',async()=>{
 await load();
 await input.focus();
 await keyboard.type('Send this');
 await keyboard.press('Enter');
 await page.waitForFunction(()=>document.querySelector('textarea').value==='');
 await input.focus();
 await keyboard.press('Control+z');
 assert.equal(await input.inputValue(),'');
});
