import { build } from 'vite';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { launchDesktopWebKit } from './webkit-driver.mjs';
import { createServer } from 'node:http';
import { mkdtemp, readFile, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const args=process.argv.slice(2);
const option=(key,fallback)=>args.includes(key)?args[args.indexOf(key)+1]:fallback;
const root=resolve(option('--source-root',fileURLToPath(new URL('..',import.meta.url))));
const output=resolve(option('--output','/tmp/solomon-rendering.json'));
const duration=Number(option('--duration-ms','10000'));
const letters=!args.includes('--no-letters');
const originalBlur=args.includes('--original-blur');
const offset=Number(option('--offset','0'));
if(!Number.isInteger(duration)||duration<100||duration>120000||!Number.isInteger(offset)||offset<0)throw new Error('Invalid recording duration or text offset');
const responsePath=option('--response',null);
const text=responsePath?JSON.parse(await readFile(responsePath,'utf8')):('# Rendering test\n\n'+('A stable paragraph with **emphasis** and wrapping text. '.repeat(15)+'\n\n').repeat(4)+'```js\nconst value = 42;\n```\n\n| Item | Value |\n| --- | --- |\n| Test | 42 |');
const directory=await mkdtemp(join(tmpdir(),'solomon-rendering-'));
const fixture=`
import React from 'react';
import {createRoot} from 'react-dom/client';
import {ChatView} from '/src/chat/ChatView.tsx';
import '/src/styles.css';
import '/src/home/welcome.css';
import '/src/theme/themes.css';
import '/src/typography.css';
window.go={main:{ServerBridge:{URL:async()=>location.origin}},app:{ServerBridge:{URL:async()=>location.origin}}};
window.__errors=[];
addEventListener('error',e=>window.__errors.push(e.message));
addEventListener('unhandledrejection',e=>window.__errors.push(String(e.reason)));
window.fetch=async()=>new Response(JSON.stringify({chat:{floatingLetters:${letters},floatingLettersDensity:'max',startReasoningCollapsed:true},projects:[],providers:[],recent:[],current:{provider:'',model:''},commands:[],suggestions:[]}),{headers:{'Content-Type':'application/json'}});
const source=${JSON.stringify(text)};
function Fixture(){
 const [content,setContent]=React.useState(source);
 const [working,setWorking]=React.useState(false);
 window.startStream=()=>{setContent(source.slice(0,${offset}));setWorking(true);let cursor=${offset};const timer=setInterval(()=>{cursor=Math.min(source.length,cursor+12);setContent(source.slice(0,cursor));if(cursor===source.length){clearInterval(timer);setWorking(false);}},50);};
 const chat={id:'diagnostic',projectID:'fixture',title:'Rendering diagnosis',status:working?'running':'done',messages:[{id:'user',role:'user',content:'Streaming test',createdAt:1},{id:'assistant',role:'assistant',content,checkpointSeq:1,workedFor:20,createdAt:1}]};
 return <ChatView chat={chat} isStreaming={working} onSend={()=>{}} onDeleteMessage={()=>{}} onStopTool={()=>{}} onStopStreaming={()=>{}}/>;
}
window.startProbe=()=>{
 window.samples=[];window.probeRunning=true;const generation=window.probeGeneration=(window.probeGeneration??0)+1;let previous=0;
 const sample=now=>{
  if(!window.probeRunning||window.probeGeneration!==generation)return;
  const response=document.querySelector('.chat-message.is-assistant .chat-response-content');
  const footer=document.querySelector('.chat-assistant-footer');
  const shell=document.querySelector('.chat-messages-shell');
  const canvas=document.querySelector('canvas');
  const rect=response?.getBoundingClientRect();
  window.samples.push({time:now,gap:previous?now-previous:0,height:rect?.height,top:rect?.top,length:response?.textContent.length,htmlLength:response?.innerHTML.length,scrollTop:shell?.scrollTop,scrollHeight:shell?.scrollHeight,footerTop:footer?.getBoundingClientRect().top,canvasHeight:canvas?.height});
  previous=now;requestAnimationFrame(sample);
 };requestAnimationFrame(sample);
};
createRoot(document.getElementById('root')).render(<Fixture/>);
`;
let browser,server;
try{
 const outDir=join(directory,'dist');
 const result=await build({root,configFile:false,logLevel:'error',cacheDir:join(directory,'cache'),plugins:[{name:'rendering-inspection',resolveId:id=>id==='/__inspect.tsx'?id:undefined,load:id=>id==='/__inspect.tsx'?fixture:undefined}],define:{__SOLOMON_GUI_BUILD_ID__:JSON.stringify('rendering-inspection')},esbuild:{jsx:'automatic'},build:{outDir,emptyOutDir:true,rollupOptions:{input:'/__inspect.tsx'},minify:true}});
 const entry=result.output.find(item=>item.type==='chunk'&&item.isEntry);
 const styles=result.output.filter(item=>item.type==='asset'&&item.fileName.endsWith('.css')).map(item=>`<link rel="stylesheet" href="/${item.fileName}">`).join('');
 const override=originalBlur?'<style>.app-shell[data-client-surface="desktop"][data-client-os="linux"] .chat-view:has(.chat-floating-letters) .chat-composer-background{backdrop-filter:blur(18px);-webkit-backdrop-filter:blur(18px);background:color-mix(in srgb,var(--color-surface-raised) 28%,transparent)}</style>':'';
 await writeFile(join(outDir,'index.html'),`<!doctype html><html><head>${styles}${override}</head><body><div id="root" class="app-shell" data-client-surface="desktop" data-client-os="linux"></div><script type="module" src="/${entry.fileName}"></script></body></html>`);
 server=createServer(async(req,res)=>{try{const path=resolve(outDir,'.'+new URL(req.url,'http://localhost').pathname);const file=path===outDir?join(outDir,'index.html'):path;res.setHeader('Access-Control-Allow-Origin','*');if(!file.startsWith(outDir+'/')){res.writeHead(403).end();return;}res.setHeader('Content-Type',file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':file.endsWith('.woff2')?'font/woff2':'text/html');res.end(await readFile(file));}catch{res.writeHead(404).end();}});
 await new Promise(done=>server.listen(0,'127.0.0.1',done));
 if(args.includes('--native')){
  const result=await promisify(execFile)('/usr/bin/python3',[fileURLToPath(new URL('./inspector.py',import.meta.url)),'--url','http://127.0.0.1:'+server.address().port+'/','--duration-ms',String(duration),'--output',output],{timeout:duration*2+45000,maxBuffer:1000000});
  console.log(result.stdout);console.log(output);
 }else{
 browser=await launchDesktopWebKit();const page=await browser.newPage();
 await page.goto('http://127.0.0.1:'+server.address().port+'/');
 await page.waitForFunction(()=>!!document.querySelector('.chat-message.is-assistant .chat-response-content')&&typeof window.startProbe==='function');
 await page.waitForFunction(expected=>Boolean(document.querySelector('canvas.chat-floating-letters'))===expected,letters);
 await page.waitForTimeout(1500);
 const config=await page.evaluate(()=>({userAgent:navigator.userAgent,letters:!!document.querySelector('canvas'),blur:getComputedStyle(document.querySelector('.chat-composer-background')).backdropFilter,devicePixelRatio,viewport:[innerWidth,innerHeight]}));
 const cases=[];
 for(const mode of ['idle','streaming']){
  await page.evaluate(mode=>{if(mode==='streaming')window.startStream();window.startProbe();},mode);
  await page.waitForTimeout(duration);
  const data=await page.evaluate(()=>{window.probeRunning=false;return {samples:window.samples,errors:window.__errors};});
  const gaps=data.samples.slice(1).map(s=>s.gap).sort((a,b)=>a-b);
  const unstable=data.samples.slice(1).flatMap((s,i)=>{const p=data.samples[i];return s.length===p.length&&s.htmlLength===p.htmlLength&&Math.abs(s.height-p.height)>1?[{before:p,after:s}]:[];});
  cases.push({mode,frames:data.samples.length,maxGap:gaps.at(-1),p95Gap:gaps[Math.floor(gaps.length*.95)],geometryChangesWithoutContent:unstable.length,examples:unstable.slice(0,10),...data});
  console.log(mode,JSON.stringify({frames:data.samples.length,maxGap:gaps.at(-1),geometryChangesWithoutContent:unstable.length,errors:data.errors}));
 }
 await writeFile(output,JSON.stringify({root,engine:browser.version(),config,duration,cases},null,2));console.log(output);
 }
}finally{
 if(browser)await browser.close();
 if(server)await new Promise(done=>server.close(done));
 await rm(directory,{recursive:true,force:true});
}
