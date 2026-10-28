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

test('dropped images share pasted previews, editing and sent attachments', async()=>{
 await load();
 await page.evaluate(()=>{
  window.setComposer('Explain these ');
  window.makeImageTransfer=names=>{
   const data=new DataTransfer();
   const bytes=Uint8Array.from(atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a3ioAAAAASUVORK5CYII='),c=>c.charCodeAt(0));
   names.forEach(name=>data.items.add(new File([bytes],name,{type:name==='dropped two.png'?'':'image/png'})));
   return data;
  };
  document.querySelector('textarea').dispatchEvent(new ClipboardEvent('paste',{clipboardData:window.makeImageTransfer(['pasted.png']),bubbles:true,cancelable:true}));
 });
 await page.waitForFunction(()=>document.querySelectorAll('.welcome-composer .composer-image-preview').length===1);
 const accepted=await page.evaluate(()=>{
  const target=document.querySelector('.welcome-toolbar');
  const data=window.makeImageTransfer(['dropped one.png','dropped two.png']);
  data.items.add(new File(['ignore this'],'notes.txt',{type:'text/plain'}));
  window.dropTransfer=data;
  const enter=new DragEvent('dragenter',{dataTransfer:data,bubbles:true,cancelable:true});
  target.dispatchEvent(enter);
  const over=new DragEvent('dragover',{dataTransfer:data,bubbles:true,cancelable:true});
  target.dispatchEvent(over);
  return {enter: enter.defaultPrevented, over: over.defaultPrevented};
 });
 assert.deepEqual(accepted,{enter:true,over:true});
 await page.waitForFunction(()=>!!document.querySelector('.is-image-drop-target'));
 assert.equal(await page.evaluate(()=>{
  const drop=new DragEvent('drop',{dataTransfer:window.dropTransfer,bubbles:true,cancelable:true});
  document.querySelector('.welcome-toolbar').dispatchEvent(drop);
  return drop.defaultPrevented;
 }),true);
 await page.waitForFunction(()=>document.querySelectorAll('.welcome-composer .composer-image-preview').length===3);
 await page.waitForFunction(()=>!document.querySelector('.is-image-drop-target'));
 assert.deepEqual(await page.evaluate(()=>Array.from(document.querySelectorAll('.welcome-composer .composer-image-preview img'),img=>img.alt)),['Open preview of pasted.png','Open preview of dropped one.png','Open preview of dropped two.png']);
 assert.equal(await page.evaluate(()=>document.querySelector('textarea').value),'Explain these [img-0] [img-1] [img-2] ');
 await page.evaluate(()=>document.querySelectorAll('.welcome-composer .composer-image-preview')[1].click());
 await page.waitForFunction(()=>!!document.querySelector('[role="dialog"][aria-label="Image preview: dropped one.png"]'));
 assert.equal(await page.evaluate(()=>!!document.querySelector('[aria-label="Image editing tools"]')),true);
 await page.evaluate(()=>document.querySelector('[role="dialog"][aria-label="Image preview: dropped one.png"]').click());
 await page.waitForFunction(()=>!document.querySelector('.composer-image-lightbox'));
 await page.evaluate(()=>window.submitComposer());
 await page.waitForFunction(()=>window.lastSentMessage?.images?.length===3);
 assert.deepEqual(await page.evaluate(()=>window.lastSentMessage.images.map(image=>({name:image.name,hasBytes:image.url.startsWith('data:image/png;base64,')}))),[
  {name:'pasted.png',hasBytes:true},{name:'dropped one.png',hasBytes:true},{name:'dropped two.png',hasBytes:true},
 ]);
 await page.waitForFunction(()=>document.querySelectorAll('.chat-message-images .composer-image-preview').length===3);
 await page.waitForFunction(()=>document.querySelectorAll('.welcome-composer .composer-image-preview').length===0);
});

test('leaving the composer cancels drop highlighting and text drops remain native', async()=>{
 await load();
 await page.evaluate(()=>{
  window.setComposer('Keep this text');
  const data=new DataTransfer();data.items.add(new File(['image'],'image.png',{type:'image/png'}));
  window.dragTransfer=data;
  document.querySelector('textarea').dispatchEvent(new DragEvent('dragenter',{dataTransfer:data,bubbles:true,cancelable:true}));
 });
 await page.waitForFunction(()=>!!document.querySelector('.is-image-drop-target'));
 await page.evaluate(()=>document.querySelector('textarea').dispatchEvent(new DragEvent('dragleave',{dataTransfer:window.dragTransfer,bubbles:true})));
 await page.waitForFunction(()=>!document.querySelector('.is-image-drop-target'));
 assert.equal(await page.evaluate(()=>{
  const data=new DataTransfer();data.setData('text/plain','ordinary text');
  const event=new DragEvent('drop',{dataTransfer:data,bubbles:true,cancelable:true});
  document.querySelector('textarea').dispatchEvent(event);
  return event.defaultPrevented;
 }),false);
 assert.equal(await page.evaluate(()=>document.querySelector('textarea').value),'Keep this text');
 assert.equal(await page.evaluate(()=>document.querySelectorAll('.welcome-composer .composer-image-preview').length),0);
});

test('native file-manager drops attach all three images instead of inserting paths', async()=>{
 for (const uriList of [true,false]) {
  await load();
  const prevented=await page.evaluate(uriList=>{
   window.setComposer('Compare ');
   const paths=['/home/oni/Pictures/Screen one.png','/home/oni/Pictures/Screen two.png','/home/oni/Pictures/Screen three.png'];
   const dataURL='data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a3ioAAAAASUVORK5CYII=';
   window.go={app:{ServerBridge:{ReadDroppedImages:async names=>{
    window.nativeDroppedPaths=names;
    return names.map(path=>({name:path.split('/').at(-1),data:dataURL}));
   }}}};
   const data=new DataTransfer();
   data.setData('text/plain',paths.join('\n'));
   if(uriList)data.setData('text/uri-list',paths.map(path=>'file://'+path.split('/').map(encodeURIComponent).join('/')).join('\r\n'));
   const drop=new DragEvent('drop',{dataTransfer:data,bubbles:true,cancelable:true});
   document.querySelector('textarea').dispatchEvent(drop);
   return drop.defaultPrevented;
  },uriList);
  assert.equal(prevented,true);
  await page.waitForFunction(()=>document.querySelectorAll('.welcome-composer .composer-image-preview').length===3);
  assert.deepEqual(await page.evaluate(()=>window.nativeDroppedPaths),['/home/oni/Pictures/Screen one.png','/home/oni/Pictures/Screen two.png','/home/oni/Pictures/Screen three.png']);
  assert.equal(await page.evaluate(()=>document.querySelector('textarea').value),'Compare [img-0] [img-1] [img-2] ');
  await page.waitForFunction(()=>[...document.querySelectorAll('.welcome-composer .composer-image-preview img')].every(image=>image.complete&&image.naturalWidth>0));
  await page.evaluate(()=>window.submitComposer());
  await page.waitForFunction(()=>window.lastSentMessage?.images?.length===3);
  assert.deepEqual(await page.evaluate(()=>window.lastSentMessage.images.map(image=>image.name)),['Screen one.png','Screen two.png','Screen three.png']);
  await page.waitForFunction(()=>document.querySelectorAll('.chat-message-images .composer-image-preview').length===3);
 }
});

test('dropping images outside the composer preserves Solomon and attaches thumbnails', async()=>{
 await load();
 const before=await page.evaluate(()=>location.href);
 const prevented=await page.evaluate(()=>{
  window.setComposer('Compare ');
  const bytes=Uint8Array.from(atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a3ioAAAAASUVORK5CYII='),c=>c.charCodeAt(0));
  const data=new DataTransfer();
  ['one.png','two.png','three.png'].forEach(name=>data.items.add(new File([bytes],name,{type:'image/png'})));
  const event=new DragEvent('drop',{dataTransfer:data,bubbles:true,cancelable:true});
  document.body.dispatchEvent(event);
  return event.defaultPrevented;
 });
 assert.equal(prevented,true);
 await page.waitForFunction(()=>document.querySelectorAll('.welcome-composer .composer-image-preview').length===3);
 assert.equal(await page.evaluate(()=>location.href),before);
 assert.equal(await page.evaluate(()=>document.querySelector('textarea').value),'Compare [img-0] [img-1] [img-2] ');
 assert.equal(await page.evaluate(()=>document.querySelectorAll('.composer-image-lightbox').length),0);
});

test('native Wails events attach images once and allow its path resolver to run', async()=>{
 await load();
 const propagated=await page.evaluate(()=>{
  window.setComposer('Native ');
  const dataURL='data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a3ioAAAAASUVORK5CYII=';
  window.go={app:{ServerBridge:{ReadDroppedImages:async paths=>paths.map(path=>({name:path.split('/').at(-1),data:dataURL}))}}};
  window.runtime={OnFileDrop:callback=>{window.nativeFileDrop=callback;},OnFileDropOff:()=>{}};
  window.restartDropWatch();
  window.bubbledDrop=false;
  window.addEventListener('drop',()=>{window.bubbledDrop=true;},{once:true});
  const data=new DataTransfer();data.items.add(new File(['bytes'],'one.png',{type:'image/png'}));
  document.body.dispatchEvent(new DragEvent('drop',{dataTransfer:data,bubbles:true,cancelable:true}));
  return window.bubbledDrop;
 });
 assert.equal(propagated,true);
 assert.equal(await page.evaluate(()=>document.querySelectorAll('.welcome-composer .composer-image-preview').length),0);
 await page.evaluate(()=>window.nativeFileDrop(10,10,['/tmp/one.png','/tmp/two.png','/tmp/three.png']));
 await page.waitForFunction(()=>document.querySelectorAll('.welcome-composer .composer-image-preview').length===3);
 assert.equal(await page.evaluate(()=>document.querySelector('textarea').value),'Native [img-0] [img-1] [img-2] ');
});

test('file drops cannot navigate the window even without an active composer', async()=>{
 await load();
 const before=await page.evaluate(()=>location.href);
 await page.evaluate(()=>window.hideComposer());
 await page.waitForFunction(()=>!document.querySelector('textarea'));
 assert.equal(await page.evaluate(()=>{
  const data=new DataTransfer();data.setData('text/uri-list','file:///tmp/image.png');
  const event=new DragEvent('drop',{dataTransfer:data,bubbles:true,cancelable:true});
  document.body.dispatchEvent(event);
  return event.defaultPrevented;
 }),true);
 assert.equal(await page.evaluate(()=>location.href),before);
});

test('real GTK file drag outside the composer keeps the app open and attaches three images', {skip:engine!=='webkitgtk'}, async()=>{
 await load();
 const png='iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a3ioAAAAASUVORK5CYII=';
 const paths=['native one.png','native two.png','native three.png'].map(name=>join(cacheDir,name));
 await Promise.all(paths.map(path=>writeFile(path,Buffer.from(png,'base64'))));
 await page.evaluate(png=>{
  window.setComposer('GTK ');
  window.go={app:{ServerBridge:{ReadDroppedImages:async paths=>paths.map(path=>({name:path.split('/').at(-1),data:'data:image/png;base64,'+png}))}}};
  window.runtime={OnFileDrop:callback=>{window.nativeFileDrop=callback;},OnFileDropOff:()=>{}};
  window.restartDropWatch();
  window.realDragEvents=[];
  for(const type of ['dragenter','dragover','drop'])window.addEventListener(type,event=>{
   window.realDragEvents.push({type,types:[...event.dataTransfer.types],uri:event.dataTransfer.getData('text/uri-list'),text:event.dataTransfer.getData('text/plain'),files:[...event.dataTransfer.files].map(file=>({name:file.name,type:file.type})),prevented:event.defaultPrevented});
  },true);
 },png);
 const before=await page.evaluate(()=>location.href);
 const result=await page.nativeDrop(paths,500,180);
 assert.equal(result.provided,true,'GTK must transfer the URI list through native Xdnd: '+JSON.stringify(result));
 assert.deepEqual(result.receivedPaths,paths,'the native GTK signal must receive the real file paths');
 const state=await page.evaluate(()=>({url:location.href,input:document.querySelector('textarea')?.value,events:window.realDragEvents,previews:document.querySelectorAll('.welcome-composer .composer-image-preview').length,body:document.body.innerText.slice(0,100)}));
 assert.equal(state.url,before,JSON.stringify(state));
 assert.equal(state.previews,3,JSON.stringify(state));
 await page.waitForFunction(()=>document.querySelectorAll('.welcome-composer .composer-image-preview').length===3);
 assert.equal(await page.evaluate(()=>location.href),before);
 assert.equal(await page.evaluate(()=>document.querySelector('textarea').value),'GTK [img-0] [img-1] [img-2] ');
 assert.equal(await page.evaluate(()=>document.querySelectorAll('.composer-image-lightbox').length),0);
});

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

test('composer uses the native caret for wrapped text, selection and Unicode',async()=>{
 await load();
 const text="non sono sicurissimo di aver capito, ma se ho capito man mano scorro l'array tenendo conto della della posizione del pivot, faccio il confronto con il valore della posizione\nAccenti: è già così. Emoji: 👩🏽‍💻. العربية";
 for(const cursor of [text.indexOf('posizione'),text.indexOf('👩'),text.length]){
  await page.evaluate(({text,cursor})=>window.setComposer(text,cursor),{text,cursor});
  await page.waitForTimeout(50);
  const state=await page.evaluate(()=>{
   const input=document.querySelector('textarea');
   return {color:getComputedStyle(input).caretColor,start:input.selectionStart,end:input.selectionEnd,focused:document.activeElement===input,overlays:document.querySelectorAll('.composer-image-caret').length};
  });
  assert.notEqual(state.color,'rgba(0, 0, 0, 0)','the real textarea caret must stay visible');
  assert.notEqual(state.color,'transparent');
  assert.equal(state.overlays,0,'a separately positioned caret can drift from native text layout');
  assert.equal(state.focused,true);
  assert.equal(state.start,cursor);
  assert.equal(state.end,cursor);
 }
 await page.evaluate(()=>document.querySelector('textarea').setSelectionRange(4,30));
 assert.deepEqual(await page.evaluate(()=>{const input=document.querySelector('textarea');return [input.selectionStart,input.selectionEnd];}),[4,30]);
});

test('composer wraps complete words and resizes without horizontal scrolling',async()=>{
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
   const input=document.querySelector('textarea'), style=getComputedStyle(input);
   const mirror=document.createElement('div');
   for(const property of ['boxSizing','font','letterSpacing','lineHeight','padding','whiteSpace','wordBreak','overflowWrap','wordSpacing','tabSize'])mirror.style[property]=style[property];
   mirror.style.position='fixed';mirror.style.width=input.clientWidth+'px';mirror.style.visibility='hidden';
   mirror.textContent=text;document.body.append(mirror);
   const wordStart=text.lastIndexOf(' ')+1;
   const range=document.createRange();range.setStart(mirror.firstChild,wordStart);range.setEnd(mirror.firstChild,text.length);
   const rects=[...range.getClientRects()];
   const bounds=mirror.getBoundingClientRect();
   const data={wordLines:rects.length,wordTop:rects[0].top-bounds.top,scrollLeft:input.scrollLeft,scrollWidth:input.scrollWidth,clientWidth:input.clientWidth};
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
 await measure(sample.prefix.length+3);
 assert.equal(await page.evaluate(()=>document.querySelector('textarea').selectionStart),sample.prefix.length+3);
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
 assert.ok(await page.evaluate(()=>{const input=document.querySelector('textarea');return input.scrollWidth<=input.clientWidth+1 && input.scrollLeft===0;}));

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
