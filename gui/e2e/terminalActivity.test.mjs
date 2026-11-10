import assert from 'node:assert/strict';
import { after, test } from 'node:test';
import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';
import { chromium } from 'playwright-core';

const fixture = `
import React from 'react';
import {createRoot} from 'react-dom/client';
import {TerminalPanel} from '/src/terminal-panel/TerminalPanel.tsx';
import '/src/styles.css';
window.activity={};
function Fixture(){
 const [open,setOpen]=React.useState(true);
 window.setOpen=setOpen;
 return <TerminalPanel height={240} maxHeight={400} isOpen={open} projectId="activity-test" workingDirectory="/workspace" onClose={()=>setOpen(false)} onHeightChange={()=>{}}
 onProjectArmedChange={(_,value)=>window.activity.visible=value} onProjectRunningChange={(_,value)=>window.activity.running=value}/>;
}
createRoot(document.getElementById('root')).render(<Fixture/>);
`;
const server=await createServer({root:fileURLToPath(new URL('..',import.meta.url)),configFile:false,esbuild:{jsx:'automatic'},server:{host:'127.0.0.1',port:0,hmr:false},plugins:[{
 name:'terminal-activity',resolveId:id=>id==='/__activity.tsx'?id:undefined,load:id=>id==='/__activity.tsx'?fixture:undefined,
 configureServer(server){server.middlewares.use((req,res,next)=>{
  if(req.url==='/__activity'){res.setHeader('Content-Type','text/html');res.end('<div id="root"></div><script type="module" src="/__activity.tsx"></script>');return;}next();
 });}
}]});
await server.listen();after(()=>server.close());
const browser=await chromium.launch({executablePath:process.env.SOLOMON_TEST_BROWSER || ['/usr/bin/chromium','/usr/bin/google-chrome'].find(existsSync),headless:true,args:['--no-sandbox']});
after(()=>browser.close());

test('idle indicators hide with the panel and executable titles stop changing after manual rename',async()=>{
 const page=await browser.newPage();
 const sockets=[];
 await page.routeWebSocket('**/__solomon/terminal?*',socket=>{
  sockets.push(socket);
  socket.send(JSON.stringify({type:'solomon-terminal',id:'activity-session'}));
  socket.send(JSON.stringify({type:'solomon-status',running:false,title:'zsh'}));
 });
 await page.goto(server.resolvedUrls.local[0]+'__activity');
 const tab=page.locator('.terminal-tab-trigger');
 await tab.filter({hasText:'zsh'}).waitFor();
 await page.waitForFunction(()=>window.activity.visible===true);
 assert.equal(await page.evaluate(()=>!!window.activity.running),false);
 await page.evaluate(()=>window.setOpen(false));
 await page.waitForFunction(()=>window.activity.visible===false);
 sockets[0].send(JSON.stringify({type:'solomon-status',running:true,title:'python'}));
 await page.waitForFunction(()=>window.activity.visible && window.activity.running);
 await page.evaluate(()=>window.setOpen(true));
 await tab.filter({hasText:'python'}).waitFor();
 const originalSize=await page.locator('.terminal-tab').evaluate(element=>({width:element.offsetWidth,height:element.offsetHeight}));
 await tab.dblclick();
 const name=page.getByRole('textbox',{name:'Terminal tab name'});
 assert.deepEqual(await page.locator('.terminal-tab').evaluate(element=>({width:element.offsetWidth,height:element.offsetHeight})),originalSize);
 await name.fill('A very long terminal name while editing');
 assert.deepEqual(await page.locator('.terminal-tab').evaluate(element=>({width:element.offsetWidth,height:element.offsetHeight})),originalSize);
 await name.fill('My service');
 await name.press('Enter');
 sockets[0].send(JSON.stringify({type:'solomon-status',running:false,title:'zsh'}));
 await page.waitForFunction(()=>window.activity.running===false);
 assert.equal(await tab.textContent(),'My service');
 assert.equal(await tab.count(),1);
 await tab.dblclick();
 await name.fill('Discarded');
 await name.press('Escape');
 assert.equal(await tab.textContent(),'My service');
 await page.evaluate(()=>window.setOpen(false));
 await page.waitForFunction(()=>window.activity.visible===false);
 await page.close();
});
