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
import {CodeEditor} from '/src/editor/CodeEditor.tsx';
import {installTextHistory} from '/src/textHistory.ts';
installTextHistory();
window.interactionErrors=[];
window.addEventListener('error',event=>window.interactionErrors.push(event.message));
function Fixture(){
 const [texts,setTexts]=React.useState({a:'First',b:'Second'});
 const [active,setActive]=React.useState('a');
 const [visible,setVisible]=React.useState(true);
 const [name,setName]=React.useState('');
 const [note,setNote]=React.useState('');
 window.setActive=setActive;window.setVisible=setVisible;window.setName=setName;
 return <><input aria-label="Name" value={name} onChange={event=>setName(event.target.value)}/><textarea aria-label="Note" value={note} onChange={event=>setNote(event.target.value)}/><textarea aria-label="Terminal" className="xterm" defaultValue=""/><input aria-label="Read only" readOnly value="Fixed"/>{visible&&<CodeEditor projectID="fixture" file={{id:active,path:active+'.txt',content:texts[active],saved:''}} onChange={content=>setTexts(current=>({...current,[active]:content}))} onCursor={()=>{}} onSave={()=>{}} onViewState={()=>{}}/>}</>;
}
createRoot(document.getElementById('root')).render(<React.StrictMode><Fixture/></React.StrictMode>);
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

const focusField=label=>page.evaluate(label=>document.querySelector('[aria-label="'+label+'"]').focus(),label);
const valueOf=label=>page.evaluate(label=>document.querySelector('[aria-label="'+label+'"]').value,label);
const shortcut=async(key,meta=false,selector)=>{
 const prevented=await page.evaluate(({key,meta,selector})=>{
 const parts=key.split('+');
 const event=new KeyboardEvent('keydown',{key:parts.at(-1),ctrlKey:!meta,metaKey:meta,shiftKey:parts.includes('Shift'),bubbles:true,cancelable:true});
 const target=selector?document.querySelector(selector):document.activeElement;
 target.dispatchEvent(event);
 return event.defaultPrevented;
},{key,meta,selector});
 await page.waitForTimeout(60);
 return prevented;
};
const insert=async text=>{
 if(page.keyboard) await page.keyboard.insertText(text);
 else await page.evaluate(text=>document.execCommand('insertText',false,text),text);
 await page.waitForTimeout(60);
};
const editorText=()=>page.evaluate(()=>document.querySelector('.cm-content').textContent);
const focusEditor=async()=>{
 await page.waitForFunction(()=>{const input=document.querySelector('.cm-content');input?.focus();return !!input&&document.activeElement===input;});
};

test('all plain text fields support Ctrl and Cmd undo with independent histories',async()=>{
 await load();
 await focusField('Name');await insert('Alice');
 await shortcut('z');assert.equal(await valueOf('Name'),'');
 await shortcut('Shift+z');assert.equal(await valueOf('Name'),'Alice');
 await focusField('Note');await insert('A note');
 await shortcut('z',true);assert.equal(await valueOf('Note'),'');
 await shortcut('Shift+z',true);assert.equal(await valueOf('Note'),'A note');
 await focusField('Name');await shortcut('z');assert.equal(await valueOf('Name'),'');
 assert.equal(await valueOf('Note'),'A note');
 await page.evaluate(()=>window.setName('External'));
 await focusField('Name');await shortcut('z');assert.equal(await valueOf('Name'),'External');
 await focusField('Terminal');assert.equal(await shortcut('z'),false);
 await focusField('Read only');assert.equal(await shortcut('z'),false);
});

test('field undo restores replaced selections and invalidates redo',async()=>{
 await load();await focusField('Name');await insert('Hello');
 await page.evaluate(()=>document.querySelector('[aria-label="Name"]').setSelectionRange(1,4));
 await insert('i');assert.equal(await valueOf('Name'),'Hio');
 await shortcut('z');assert.equal(await valueOf('Name'),'Hello');
 assert.deepEqual(await page.evaluate(()=>{const input=document.querySelector('[aria-label="Name"]');return [input.selectionStart,input.selectionEnd];}),[1,4]);
 await insert('ey');const current=await valueOf('Name');await shortcut('y');assert.equal(await valueOf('Name'),current);
});

test('file editor supports undo redo and retains separate tab histories across remounts',async()=>{
 await load();await page.waitForFunction(()=>!!document.querySelector('.cm-content'));
 await focusEditor();await insert('One');assert.equal(await editorText(),'OneFirst');
 await shortcut('z',false,'.cm-content');assert.equal(await editorText(),'First');
 await shortcut('Shift+z',false,'.cm-content');assert.equal(await editorText(),'OneFirst');
 await page.evaluate(()=>window.setActive('b'));await page.waitForFunction(()=>document.querySelector('.cm-content').textContent==='Second');
 await focusEditor();await insert('Two');await shortcut('z',false,'.cm-content');assert.equal(await editorText(),'Second');
 await page.evaluate(()=>window.setActive('a'));await page.waitForFunction(()=>document.querySelector('.cm-content').textContent==='OneFirst');
 await focusEditor();await shortcut('z',false,'.cm-content');assert.equal(await editorText(),'First');
 await page.evaluate(()=>window.setVisible(false));await page.waitForFunction(()=>!document.querySelector('.cm-content'));
 await page.evaluate(()=>window.setVisible(true));await page.waitForFunction(()=>!!document.querySelector('.cm-content'));
 await focusEditor();await shortcut('y',false,'.cm-content');assert.equal(await editorText(),'OneFirst');
});
