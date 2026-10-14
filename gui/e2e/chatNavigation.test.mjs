import assert from 'node:assert/strict';
import {after,test} from 'node:test';
import {existsSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
import {createServer} from 'vite';
import react from '@vitejs/plugin-react';
import {chromium} from 'playwright-core';
const fixture=`import React from 'react'; import {createRoot} from 'react-dom/client';
import {ChatView} from '/src/chat/ChatView.tsx';
import '/src/styles.css';import '/src/home/welcome.css';import '/src/chat/chat.css';
window.sent=[];window.failSend=false;window.holdSend=false;
const messages=Array.from({length:60},(_,index)=>({id:String(index),role:index%2?'assistant':'user',content:'Message '+index+'\\nSecond line\\nThird line'}));
function Fixture(){
 const [visible,setVisible]=React.useState(true),[id,setID]=React.useState('a'),[projectID,setProject]=React.useState('project-a'),[extra,setExtra]=React.useState(0);
 window.addContent=()=>setExtra(n=>n+1);
 const chat={id,projectID,title:id,messages:extra?[...messages,{id:'extra',role:'assistant',content:'New streaming content '.repeat(extra)}]:messages};
 const send=async(id,text)=>{if(window.failSend)throw new Error('Send unavailable');window.sent.push({id,text});if(window.holdSend)await new Promise(resolve=>window.finishSend=resolve)};
 return React.createElement(React.Fragment,null,
 React.createElement('nav',{style:{position:'fixed',zIndex:100,top:0,right:0}},
 ...['a','b'].map(id=>React.createElement('button',{id:'chat-'+id,key:id,onClick:()=>{setVisible(true);setID(id)}},'Chat '+id)),
 React.createElement('button',{id:'editor',onClick:()=>setVisible(false)},'Editor'),React.createElement('button',{id:'agent',onClick:()=>setVisible(true)},'Agent'),
 React.createElement('button',{id:'project-a',onClick:()=>setProject('project-a')},'Project A'),React.createElement('button',{id:'project-b',onClick:()=>setProject('project-b')},'Project B')),
 visible?React.createElement(ChatView,{key:projectID+id,chat,onSend:send,onDeleteMessage(){},onStopTool(){},onStopStreaming(){}}):null);
}
createRoot(document.getElementById('root')).render(React.createElement(React.StrictMode,null,React.createElement(Fixture)));
`;
const server=await createServer({root:fileURLToPath(new URL('..',import.meta.url)),configFile:false,plugins:[react(),{name:'navigation',resolveId:id=>id==='/__navigation.jsx'?id:null,load:id=>id==='/__navigation.jsx'?fixture:null}],server:{host:'127.0.0.1',port:0,hmr:false}});
await server.listen();after(()=>server.close());
const browser=await chromium.launch({executablePath:['/usr/bin/chromium','/usr/bin/google-chrome'].find(existsSync),headless:true,args:['--no-sandbox']});after(()=>browser.close());
async function openPage(t){
 const context=await browser.newContext({viewport:{width:1200,height:800}});t.after(()=>context.close());
 const page=await context.newPage();page.setDefaultTimeout(7000);
 const errors=[];page.on('pageerror',error=>errors.push(error.message));t.after(()=>assert.deepEqual(errors,[]));
 await page.route('**/__navigation',route=>route.fulfill({contentType:'text/html',body:'<div id="root" class="app-shell"></div><script type="module" src="/__navigation.jsx"></script>'}));
 await page.route('**/__solomon/**',route=>route.fulfill({json:{projects:[],chat:{startToolCallsCollapsed:false,autoCloseToolCalls:false,startReasoningCollapsed:false},current:{provider:'',model:''},providers:[],recent:[]}}));
 await page.goto(server.resolvedUrls.local[0]+'__navigation');await input(page).waitFor();return page;
}
const input=page=>page.getByRole('textbox',{name:'Message',exact:true});
test('drafts survive view/chat/project changes and reload; clear only after successful send',async t=>{
 const page=await openPage(t);await input(page).fill('Draft A\nKeep whitespace  ');
 await page.locator('#editor').click();await page.locator('#agent').click();assert.equal(await input(page).inputValue(),'Draft A\nKeep whitespace  ');
 await page.locator('#chat-b').click();assert.equal(await input(page).inputValue(),'');await input(page).fill('Draft B');
 await page.locator('#chat-a').click();assert.equal(await input(page).inputValue(),'Draft A\nKeep whitespace  ');
 await page.locator('#project-b').click();assert.equal(await input(page).inputValue(),'');await input(page).fill('Other project');
 await page.locator('#project-a').click();assert.equal(await input(page).inputValue(),'Draft A\nKeep whitespace  ');
 await page.reload();assert.equal(await input(page).inputValue(),'Draft A\nKeep whitespace  ');
 await page.evaluate(()=>window.failSend=true);await page.getByRole('button',{name:'Send',exact:true}).click();
 await page.locator('#chat-b').click();assert.equal(await input(page).inputValue(),'Draft B');
 await page.locator('#chat-a').click();assert.equal(await input(page).inputValue(),'Draft A\nKeep whitespace  ');
 await page.evaluate(()=>window.failSend=false);await page.getByRole('button',{name:'Send',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('textarea')?.value==='');
 await page.locator('#chat-b').click();assert.equal(await input(page).inputValue(),'Draft B');
 await page.locator('#chat-a').click();assert.equal(await input(page).inputValue(),'');
});
test('async sends cannot erase another chat or a newer draft',async t=>{
 const page=await openPage(t);await input(page).fill('Message to send');await page.evaluate(()=>window.holdSend=true);
 await page.getByRole('button',{name:'Send',exact:true}).click();await page.waitForFunction(()=>typeof window.finishSend==='function');
 await page.locator('#chat-b').click();await input(page).fill('Keep B');await page.evaluate(()=>window.finishSend());
 assert.equal(await input(page).inputValue(),'Keep B');await page.locator('#chat-a').click();assert.equal(await input(page).inputValue(),'');
 await input(page).fill('Second message');await page.getByRole('button',{name:'Send',exact:true}).click();await page.waitForFunction(()=>window.sent.length===2);
 await input(page).fill('Newer draft');await page.evaluate(()=>window.finishSend());
 await page.locator('#editor').click();await page.locator('#agent').click();assert.equal(await input(page).inputValue(),'Newer draft');
});
test('chat reading positions survive Editor and chat switches without following new messages',async t=>{
 const page=await openPage(t);const shell=page.locator('.chat-messages-shell');
 const bottom=()=>{const el=document.querySelector('.chat-messages-shell');return el&&el.scrollHeight-el.scrollTop-el.clientHeight<2};
 await page.waitForFunction(bottom);await shell.hover({position:{x:300,y:120}});await page.mouse.wheel(0,-1400);
 await page.waitForFunction(()=>document.querySelector('.chat-scroll-to-bottom')?.classList.contains('is-visible'));
 const top=await shell.evaluate(node=>node.scrollTop);assert.ok(top>0);
 await page.locator('#editor').click();await page.locator('#agent').click();
 await page.waitForFunction(top=>Math.abs(document.querySelector('.chat-messages-shell').scrollTop-top)<2,top);
 await page.evaluate(()=>window.addContent());await page.waitForFunction(()=>document.querySelector('.chat-messages')?.textContent.includes('New streaming content'));
 assert.ok(Math.abs(await shell.evaluate(node=>node.scrollTop)-top)<2);
 await page.locator('#chat-b').click();await page.waitForFunction(bottom);await shell.hover({position:{x:300,y:120}});await page.mouse.wheel(0,-600);
 await page.waitForFunction(()=>document.querySelector('.chat-scroll-to-bottom')?.classList.contains('is-visible'));
 const topB=await shell.evaluate(node=>node.scrollTop);await page.locator('#chat-a').click();
 await page.waitForFunction(top=>Math.abs(document.querySelector('.chat-messages-shell').scrollTop-top)<2,top);
 await page.locator('#chat-b').click();await page.waitForFunction(top=>Math.abs(document.querySelector('.chat-messages-shell').scrollTop-top)<2,topB);
 await page.getByRole('button',{name:'Scroll to bottom',exact:true}).click();await page.waitForFunction(bottom);
 await page.locator('#editor').click();await page.locator('#agent').click();await page.waitForFunction(bottom);
});


test('draft image URLs and terminal clips remain usable after navigation',async t=>{
 const page=await openPage(t);
 await page.getByLabel('Choose images').setInputFiles({name:'draft.png',mimeType:'image/png',buffer:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=','base64')});
 await page.evaluate(()=>window.dispatchEvent(new CustomEvent('solomon-terminal-clip',{detail:{start:1,end:2,tag:'[terminal-L1-L2]',text:'Keep terminal output'}})));
 const text=await input(page).inputValue();
 const imageURL=await page.locator('.composer-image-preview img').first().getAttribute('src');
 assert.ok(imageURL?.startsWith('blob:'));
 await page.locator('#chat-b').click();await page.locator('#chat-a').click();
 await page.locator('#editor').click();await page.locator('#agent').click();
 assert.equal(await input(page).inputValue(),text);
 assert.equal(await page.locator('.composer-image-preview img').first().getAttribute('src'),imageURL);
 assert.ok(await page.evaluate(async url=>(await fetch(url)).ok,imageURL));
 await page.getByLabel('Choose images').setInputFiles({name:'second.png',mimeType:'image/png',buffer:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=','base64')});
 assert.equal(await page.locator('.composer-image-preview img').count(),2);
 assert.ok((await input(page).inputValue()).includes('[img-1]'));
 await page.getByRole('button',{name:'Send',exact:true}).click();
 await page.waitForFunction(()=>window.sent.length===1);
 const sent=await page.evaluate(()=>window.sent[0].text);
 assert.equal(sent.clips[0].text,'Keep terminal output');
 assert.equal(sent.images[0].name,'draft.png');
 await page.waitForFunction(()=>document.querySelector('textarea')?.value==='');
});
