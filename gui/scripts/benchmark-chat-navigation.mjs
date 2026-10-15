import { build } from "vite";
import { chromium } from "playwright-core";
import { launchDesktopWebKit } from "./webkit-driver.mjs";
import { createServer } from "node:http";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const args = process.argv.slice(2);
const option = (key, fallback) => args.includes(key) ? args[args.indexOf(key) + 1] : fallback;
const sizes = option("--messages", "20,100,300").split(",").map(Number);
const engine = option("--engine", "chromium");
if (!["chromium", "webkitgtk"].includes(engine)) throw new Error("Engine must be chromium or webkitgtk");
const interactions = args.includes("--interactions");
const withTools = args.includes("--tools");
const floatingLetters = args.includes("--floating-letters");
const repeats = Number(option("--repeats", "6"));
const serverDelayMs = Number(option("--server-delay", "50"));
const maxCachedMs = Number(option("--max-cached-ms", "0"));
const minAnimationFPS = Number(option("--min-animation-fps", "0"));
const maxAnimationGapMs = Number(option("--max-animation-gap-ms", "0"));
const animationSampleMs = Number(option("--animation-sample-ms", "0"));
const output = resolve(option("--output", "chat-navigation-benchmark.json"));
if (![maxCachedMs,minAnimationFPS,maxAnimationGapMs,animationSampleMs].every(value=>Number.isFinite(value)&&value>=0) || sizes.some(size => !Number.isInteger(size) || size < 1 || size > 2000) || !Number.isInteger(repeats) || repeats < 1 || repeats > 100 || !Number.isFinite(serverDelayMs) || serverDelayMs < 0) throw new Error("Invalid benchmark arguments");
const directory = await mkdtemp(join(tmpdir(), "solomon-chat-benchmark-"));
const input = "/__benchmark.tsx";
const outDir = join(directory, "dist");
let browser, server;
const fixture = `
import React from 'react';
import { createRoot } from 'react-dom/client';
import { ChatView } from '/src/chat/ChatView.tsx';
import { useChatRuntime } from '/src/chat/useChatRuntime.ts';
import { clearChatStore, updateChat } from '/src/chat/chatStore.ts';
import { applyLiveStreamEvent } from '/src/chat/chatClient.ts';
import { navigationReport, clearNavigationPerformance } from '/src/chat/navigationPerformance.ts';
import '/src/styles.css';
import '/src/home/welcome.css';
import '/src/theme/themes.css';
import '/src/typography.css';
window.fixtureChats = {};
window.go = {main:{ServerBridge:{URL:async()=>location.origin}}};
window.cardRectReads = 0;
window.flightProbe = null;
const originalRangeRect = Range.prototype.getBoundingClientRect;
Range.prototype.getBoundingClientRect = function(...args) {
  if(window.flightProbe) window.flightProbe.rangeReads++;
  return originalRangeRect.apply(this,args);
};
const originalClear = CanvasRenderingContext2D.prototype.clearRect;
const originalRestore = CanvasRenderingContext2D.prototype.restore;
CanvasRenderingContext2D.prototype.clearRect = function(...args) {
  if(window.flightProbe && this.canvas.classList.contains('chat-floating-letters')) {
    const probe=window.flightProbe, now=performance.now();
    if(probe.started) probe.frameGaps.push(now-probe.started);
    probe.started=now;
  }
  return originalClear.apply(this,args);
};
CanvasRenderingContext2D.prototype.restore = function(...args) {
  const result=originalRestore.apply(this,args);
  if(window.flightProbe && this.canvas.classList.contains('chat-floating-letters')) {
    window.flightProbe.drawTimes.push(performance.now()-window.flightProbe.started);
  }
  return result;
};
const originalRect = HTMLElement.prototype.getBoundingClientRect;
HTMLElement.prototype.getBoundingClientRect = function() {
  if (this.matches('.chat-message.is-user, .chat-tool-card, .chat-subagent-card')) window.cardRectReads++;
  return originalRect.call(this);
};
window.fetch = async url => {
  await new Promise(resolve => setTimeout(resolve, ${serverDelayMs}));
  const path = new URL(url, location.href).pathname;
  const id = path.split('/').at(-1);
  const data = window.fixtureChats[id] ?? (path.endsWith('/branches') ? {isRepo:true,current:'main',branches:['main']} : path.endsWith('/worktrees') ? {worktrees:[]} : {projects:[], chat:{floatingLetters:${floatingLetters},floatingLettersDensity:"high",startToolCallsCollapsed:false,autoCloseToolCalls:false,startReasoningCollapsed:false},current:{provider:'',model:''},providers:[],recent:[]});
  return new Response(JSON.stringify(data), {headers:{'Content-Type':'application/json'}});
};
function Fixture() {
  const runtime = useChatRuntime();
  const [visible, setVisible] = React.useState(true);
  const [visualWorking, setVisualWorking] = React.useState(false);
  window.queueTokenBurst = suffix => {
    setVisualWorking(true);
    updateChat(runtime.selectedChat.id, chat => ({...chat,messages:chat.messages.map((message,index)=>index===chat.messages.length-1?{...message,content:message.content+'\\n\\n'+suffix}:message)}));
  };
  window.finishTokenStream = () => setVisualWorking(false);
  window.persistVisualResponse = () => {
    setVisualWorking(false);
    updateChat(runtime.selectedChat.id, chat => applyLiveStreamEvent(chat, {type:'chat_snapshot',chat:{...chat,mode:'agent',status:'done',messages:chat.messages.map((message,index)=>index===chat.messages.length-1?{...message,id:message.id+'-saved'}:message)}}));
  };
  window.appendRawTokenText = suffix => updateChat(runtime.selectedChat.id, chat => ({...chat,messages:chat.messages.map((message,index)=>index===chat.messages.length-1?{...message,content:message.content+suffix}:message)}));
  window.appendResponse = () => updateChat(runtime.selectedChat.id, chat => ({...chat,messages:chat.messages.map((message,index)=>index===chat.messages.length-1?{...message,content:message.content+' New streamed text.'}:message)}));
  window.openChat = id => { void runtime.openProjectChat({id:'benchmark',name:'Benchmark',path:'/tmp/benchmark'}, id); };
  window.configure = (size, key) => {
    runtime.clearSelection(); clearChatStore(); clearNavigationPerformance();
    window.fixtureChats = {};
    for (const suffix of ['a','b']) {
      const id = key + '-' + suffix;
      const messages = Array.from({length:size}, (_,index) => ({id:String(index),checkpointSeq:index,role:index%2?'assistant':'user',toolCalls:${withTools}?index%2?[{id:'tool-'+index,name:'shell',intent:'Inspect step '+index,input:'echo step-'+index,status:'success',defaultOpen:false,result:{status:'success',output:'Result '+index}}]:undefined:undefined,content:index%2?
        '## Response ' + index + '\\n\\n' + 'A paragraph with **bold**, \`inline code\`, and [a link](https://example.com). '.repeat(12) + '\\n\\n~~~js\\nconst result = { value: 42 };\\nconsole.log(result);\\n~~~\\n\\n| Key | Value |\\n| --- | --- |\\n| result | 42 |' : 'Please explain step ' + index}));
      window.fixtureChats[id] = {id,title:id,mode:'agent',status:'done',messages};
    }
  };
  window.report = navigationReport;
  return <><nav style={{position:'fixed',right:10,top:0,zIndex:100}}>
    {Object.keys(window.fixtureChats).map(id=><button data-chat-id={id} key={id} onClick={()=>window.openChat(id)}>{id}</button>)}
    <button id="editor" onClick={()=>setVisible(false)}>Editor</button><button id="agent" onClick={()=>setVisible(true)}>Agent</button>
  </nav>{runtime.selectedChat && visible ? <ChatView key={runtime.selectedChat.id} chat={visualWorking?{...runtime.selectedChat,status:"running"}:runtime.selectedChat} isStreaming={visualWorking}
    onSend={runtime.sendMessage} onDeleteMessage={runtime.deleteMessage} onStopTool={runtime.stopTool} onStopStreaming={runtime.stopChat} /> : <p>Waiting</p>}</>;
}
createRoot(document.getElementById('root')).render(<Fixture />);
`;
try {
  const bundle = await build({ root, configFile: false, logLevel: "error",
    plugins: [{ name: "benchmark-fixture", resolveId: id => id === "/__benchmark.tsx" ? id : undefined, load: id => id === "/__benchmark.tsx" ? fixture : undefined }],
    define: { __SOLOMON_GUI_BUILD_ID__: JSON.stringify(`benchmark-${new Date().toISOString()}`) },
    esbuild: { jsx: "automatic" }, build: { outDir, emptyOutDir: true, rollupOptions: { input }, minify: true },
  });
  const entry = bundle.output.find(item => item.type === "chunk" && item.isEntry);
  if (!entry) throw new Error("Benchmark entry was not emitted");
  const styles = bundle.output.filter(item => item.type === "asset" && item.fileName.endsWith(".css"))
    .map(item => `<link rel="stylesheet" href="/${item.fileName}">`).join("");
  await writeFile(join(outDir, "index.html"), `<!doctype html><html><head>${styles}</head><body><div id="root" class="app-shell"></div><script type="module" src="/${entry.fileName}"></script></body></html>`);
  server = createServer(async (request, response) => {
    try {
      const pathname = new URL(request.url, "http://localhost").pathname;
      const file = pathname === "/" ? join(outDir, "index.html") : join(outDir, pathname);
      if (!resolve(file).startsWith(resolve(outDir) + "/")) { response.writeHead(403).end(); return; }
      response.setHeader("Content-Type", file.endsWith(".js") ? "text/javascript" : file.endsWith(".css") ? "text/css" : "text/html");
      response.end(await readFile(file));
    } catch { response.writeHead(404).end(); }
  });
  await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
  const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/google-chrome"].find(existsSync);
  if (!executablePath) throw new Error("Install Chromium or set SOLOMON_TEST_BROWSER");
  browser = engine === "webkitgtk" ? await launchDesktopWebKit() : await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  page.setDefaultTimeout(60000);
  const errors = [];
  page.on("pageerror", error => errors.push(error.message));
  const fixtureURL = `http://127.0.0.1:${server.address().port}/`;
  await page.goto(fixtureURL);
  await page.waitForFunction(() => typeof window.configure === "function");
  const cases = [];
  const budgetFailures = [];
  for (const size of sizes) {
    const key = `messages-${size}`;
    await page.evaluate(({ size, key }) => window.configure(size, key), { size, key });
    for (let index = 0; index < repeats + 2; index++) {
      const id = `${key}-${index % 2 ? "b" : "a"}`;
      const before = await page.evaluate(() => window.report().samples.length);
      await page.evaluate(id => window.openChat(id), id);
      await page.waitForFunction(before => {
        const samples = window.report().samples;
        const sample = samples.at(-1);
        return samples.length > before && sample?.paintOpportunityMs !== undefined && sample.spans.some(span => span.phase === 'chat.total') && sample.spans.some(span => span.phase === 'git.worktrees');
      }, before);
      // Include delayed syntax highlighting and long-task observer delivery.
      await page.waitForTimeout(500);
    }
    const report = await page.evaluate(() => window.report());
    const summary = cached => {
      const values = report.samples.filter(sample => sample.cached === cached && sample.outcome === "ready").map(sample => sample.paintOpportunityMs).sort((a, b) => a - b);
      return { count: values.length, medianMs: values[Math.ceil(values.length * .5) - 1], p95Ms: values[Math.ceil(values.length * .95) - 1], maxMs: values.at(-1) };
    };
    let interactionResults;
    let flightPerformance;
    if (interactions) {
      await page.evaluate(() => {
        const input = document.querySelector('textarea[aria-label="Message"]');
        Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value').set.call(input, 'Draft during animation');
        input.dispatchEvent(new Event('input', {bubbles:true}));
      });
      const initialID = await page.evaluate(() => document.querySelector('.chat-view').getAttribute('aria-label').replace('Chat: ', ''));
      const otherID = initialID.endsWith('-a') ? key+'-b' : key+'-a';
      for (const id of [otherID, initialID]) {
        await page.evaluate(id => document.querySelector('[data-chat-id="'+id+'"]').click(), id);
        await page.waitForFunction(id => document.querySelector('.chat-view')?.getAttribute('aria-label') === 'Chat: '+id, id);
      }
      await page.waitForFunction(() => document.querySelector('textarea[aria-label="Message"]')?.value === 'Draft during animation');
      if (withTools) {
        const before = await page.evaluate(() => document.querySelector('.chat-tool-card details').open);
        await page.evaluate(() => document.querySelector('.chat-tool-card summary').click());
        await page.waitForFunction(before => document.querySelector('.chat-tool-card details').open !== before, before);
        await page.evaluate(() => document.querySelector('.chat-tool-card summary').click());
        await page.waitForFunction(before => document.querySelector('.chat-tool-card details').open === before, before);
      }
      if (floatingLetters) {
        const covered = await page.evaluate(() => {
          const layer=document.querySelector('canvas.chat-floating-letters').getBoundingClientRect();
          const composer=document.querySelector('.chat-composer .welcome-composer').getBoundingClientRect();
          return layer.top<composer.top && layer.bottom>=composer.bottom && layer.left<composer.left && layer.right>composer.right;
        });
        if (!covered) throw new Error('Floating letters leave the composer side bands uncovered');
      }
      // Wait for initial following-bottom restoration before simulating input.
      await page.waitForFunction(() => {const el=document.querySelector('.chat-messages-shell');return el.scrollHeight-el.scrollTop-el.clientHeight<2;});
      await page.evaluate(() => {
        const shell = document.querySelector('.chat-messages-shell');
        shell.dispatchEvent(new WheelEvent('wheel',{deltaY:-1000,bubbles:true}));
        shell.scrollTop = shell.scrollHeight / 2;
      });
      await page.waitForFunction(() => document.querySelector('.chat-scroll-to-bottom').classList.contains('is-visible'));
      const scrollTop = await page.evaluate(() => document.querySelector('.chat-messages-shell').scrollTop);
      await page.evaluate(() => window.appendResponse());
      await page.waitForTimeout(100);
      const afterAppend = await page.evaluate(() => document.querySelector('.chat-messages-shell').scrollTop);
      if (Math.abs(afterAppend-scrollTop)>2) throw new Error('Streaming displaced the reading position');
      let historyPrepend = false;
      if (await page.evaluate(() => Boolean(document.querySelector('.chat-load-earlier')))) {
        const anchor = await page.evaluate(() => {
          const shell = document.querySelector('.chat-messages-shell').getBoundingClientRect();
          const element = [...document.querySelectorAll('.chat-response-content')].find(el=>el.getBoundingClientRect().top>shell.top+30);
          return {id:element.dataset.messageId,top:element.getBoundingClientRect().top,count:document.querySelectorAll('.chat-response-content').length};
        });
        await page.evaluate(() => document.querySelector('.chat-load-earlier').click());
        await page.waitForFunction(count => document.querySelectorAll('.chat-response-content').length>count,anchor.count);
        const anchorTop = await page.evaluate(id => document.querySelector('.chat-response-content[data-message-id="'+id+'"]').getBoundingClientRect().top,anchor.id);
        if (Math.abs(anchorTop-anchor.top)>2) throw new Error('Prepending history displaced the reading anchor');
        historyPrepend = true;
      }
      // Reach the historical boundary using scrolling, not the load button.
      let automaticHistory = false;
      if (await page.evaluate(() => Boolean(document.querySelector('.chat-load-earlier')))) {
        const count = await page.evaluate(() => document.querySelectorAll('.chat-response-content').length);
        await page.evaluate(() => {
          const shell=document.querySelector('.chat-messages-shell');
          shell.dispatchEvent(new WheelEvent('wheel',{deltaY:-1000,bubbles:true}));
          shell.scrollTop=0;
        });
        await page.waitForFunction(count=>document.querySelectorAll('.chat-response-content').length>count,count);
        automaticHistory = true;
      }
      await page.evaluate(() => document.querySelector('.chat-scroll-to-bottom').click());
      await page.waitForFunction(() => {const el=document.querySelector('.chat-messages-shell');return el.scrollHeight-el.scrollTop-el.clientHeight<2;});
      await page.evaluate(() => document.querySelector('#editor').click());
      await page.waitForFunction(() => !document.querySelector('.chat-view'));
      await page.evaluate(() => document.querySelector('#agent').click());
      await page.waitForFunction(() => document.querySelector('textarea[aria-label="Message"]')?.value === 'Draft during animation');
      await page.evaluate(({key}) => {
        for(const suffix of ['a','b','a','b']) document.querySelector('[data-chat-id="'+key+'-'+suffix+'"]').click();
      },{key});
      await page.waitForFunction(key => document.querySelector('.chat-view')?.getAttribute('aria-label') === 'Chat: '+key+'-b',key);
      await page.waitForTimeout(500);
      let tokenPacing = false;
      if (floatingLetters) {
        await page.evaluate(()=>{
          window.idleCadence=null;
          const timestamps=[];
          const sample=now=>{
            timestamps.push(now);
            if(now-timestamps[0]<1000) requestAnimationFrame(sample);
            else window.idleCadence={fps:1000*(timestamps.length-1)/(now-timestamps[0]),frameIntervalMs:(now-timestamps[0])/(timestamps.length-1)};
          };
          requestAnimationFrame(sample);
        });
        await page.waitForFunction(()=>window.idleCadence!==null);
        const burst = 'PacedTokens0123456789'.repeat(Math.max(4,Math.ceil(animationSampleMs/250)));
        await page.evaluate(burst => {
          const response=[...document.querySelectorAll('.chat-response-content')].at(-1);
          window.tokenSnapshots=[response.textContent];
          window.flightProbe={rangeReads:0,started:0,frameGaps:[],drawTimes:[]};
          window.tokenObserver=new MutationObserver(()=>window.tokenSnapshots.push(response.textContent));
          window.tokenObserver.observe(response,{childList:true,characterData:true,subtree:true});
          window.queueTokenBurst(burst);
        },burst);
        await page.waitForFunction(() => window.tokenSnapshots.length>1);
        const premature = await page.evaluate(burst=>window.tokenSnapshots.at(-1).endsWith(burst),burst);
        if(premature) throw new Error('A fast token burst appeared all at once');
        // The final daemon snapshot replaces the temporary streaming ID while
        // visual tokens are still queued. This must never rewind presentation.
        await page.evaluate(()=>{
          window.tokenObserver.disconnect();
          window.tokenObserver=new MutationObserver(()=>window.tokenSnapshots.push([...document.querySelectorAll('.chat-response-content')].at(-1).textContent));
          window.tokenObserver.observe(document.querySelector('.chat-messages-shell'),{childList:true,characterData:true,subtree:true});
          window.persistVisualResponse();
        });
        await page.waitForFunction(burst=>[...document.querySelectorAll('.chat-response-content')].at(-1).textContent.endsWith(burst),burst);
        const growth = await page.evaluate(()=>{
          window.tokenObserver.disconnect();
          return Math.max(...window.tokenSnapshots.slice(1).map((text,index)=>text.length-window.tokenSnapshots[index].length));
        });
        if(growth>8) throw new Error('Token pacing revealed a large batch: '+growth);
        flightPerformance=await page.evaluate(()=>{
          const probe=window.flightProbe;window.flightProbe=null;
          const percentile=(values,fraction)=>values.sort((a,b)=>a-b)[Math.min(values.length-1,Math.ceil(values.length*fraction)-1)]??0;
          const elapsed=probe.frameGaps.reduce((sum,gap)=>sum+gap,0);
          return {idleFPS:window.idleCadence.fps,fps:1000*probe.frameGaps.length/Math.max(1,elapsed),durationMs:elapsed,frames:probe.drawTimes.length,rangeReads:probe.rangeReads,rangeReadsPerFrame:probe.rangeReads/Math.max(1,probe.drawTimes.length),drawP95Ms:percentile(probe.drawTimes,.95),frameGapP95Ms:percentile(probe.frameGaps,.95),frameGapP99Ms:percentile(probe.frameGaps,.99),frameGapMaxMs:Math.max(0,...probe.frameGaps),framesOver25Ms:probe.frameGaps.filter(gap=>gap>25).length,framesOver50Ms:probe.frameGaps.filter(gap=>gap>50).length};
        });
        const rewind = await page.evaluate(()=>window.tokenSnapshots.some((text,index)=>index>0 && text.length<window.tokenSnapshots[index-1].length));
        if(rewind) throw new Error('Saving the streamed message restarted its token animation');
        await page.waitForTimeout(600);
        await page.evaluate(()=>{
          window.beforeSavedSnapshot=[...document.querySelectorAll('.chat-response-content')].at(-1).textContent;
          window.persistVisualResponse();
        });
        await page.waitForTimeout(150);
        const savedStable = await page.evaluate(()=>[...document.querySelectorAll('.chat-response-content')].at(-1).textContent===window.beforeSavedSnapshot && Array.from(CSS.highlights.values()).every(value=>value.size===0));
        if(!savedStable) throw new Error('A saved snapshot replayed settled letters');
        // Leaving a view must cancel its pending presentation queue. Reopening
        // it shows the stored transcript without replaying historical tokens.
        const leavingBurst = 'QueueDuringNavigation'.repeat(4);
        await page.evaluate(text=>window.queueTokenBurst(text),leavingBurst);
        await page.evaluate(()=>document.querySelector('#editor').click());
        await page.waitForFunction(()=>!document.querySelector('.chat-view'));
        await page.evaluate(()=>document.querySelector('#agent').click());
        await page.waitForFunction(text=>[...document.querySelectorAll('.chat-response-content')].at(-1)?.textContent.endsWith(text),leavingBurst);
        await page.evaluate(()=>window.finishTokenStream());
        await page.evaluate(()=>window.queueTokenBurst('**NoReplayLetters'));
        await page.waitForFunction(()=>[...document.querySelectorAll('.chat-response-content')].at(-1).textContent.endsWith('**NoReplayLetters'));
        await page.waitForTimeout(600);
        await page.evaluate(()=>{
          const context=document.querySelector('canvas.chat-floating-letters').getContext('2d');
          window.replayedGlyphs=0;
          window.originalLetterPaint=context.fillText;
          context.fillText=function(...args){
            if(this.globalAlpha>0.2) window.replayedGlyphs++;
            return window.originalLetterPaint.apply(this,args);
          };
          window.appendRawTokenText('**');
        });
        await page.waitForFunction(()=>[...document.querySelectorAll('.chat-response-content strong')].some(element=>element.textContent==='NoReplayLetters'));
        await page.waitForTimeout(150);
        const replayed = await page.evaluate(()=>{
          document.querySelector('canvas.chat-floating-letters').getContext('2d').fillText=window.originalLetterPaint;
          return window.replayedGlyphs;
        });
        if(replayed) throw new Error('Settled Markdown letters replayed their flight: '+replayed);
        await page.evaluate(()=>window.finishTokenStream());
        for (const [index,[delimiter,selector]] of [['**','strong'],['*','em'],['__','strong'],['_','em'],['~~','del'],['`','code']].entries()) {
          const word='Inline'+index+'abcdefghijk'.repeat(4);
          await page.evaluate(text=>window.queueTokenBurst(text),delimiter+word);
          await page.waitForFunction(text=>[...document.querySelectorAll('.chat-response-content')].at(-1).textContent.endsWith(text),delimiter+word);
          await page.evaluate(({delimiter,selector,word})=>{
            const response=[...document.querySelectorAll('.chat-response-content')].at(-1);
            window.inlineFlightSnapshot=null;
            const observer=new MutationObserver(()=>{
              if(![...response.querySelectorAll(selector)].some(element=>element.textContent===word)) return;
              const ranges=Array.from(CSS.highlights.values()).flatMap(value=>Array.from(value));
              window.inlineFlightSnapshot={count:ranges.length,invalid:ranges.filter(range=>!range.startContainer.isConnected || !range.toString()).length};
              observer.disconnect();
            });
            observer.observe(response,{childList:true,characterData:true,subtree:true});
            window.appendRawTokenText(delimiter);
          },{delimiter,selector,word});
          await page.waitForFunction(()=>window.inlineFlightSnapshot!==null);
          const snapshot=await page.evaluate(()=>window.inlineFlightSnapshot);
          if(!snapshot.count || snapshot.invalid) throw new Error('Inline formatting detached in-flight highlights: '+JSON.stringify(snapshot));
        }
        await page.evaluate(()=>window.finishTokenStream());
        tokenPacing=true;
      }
      await page.waitForTimeout(500);
      await page.evaluate(() => { window.cardRectReads = 0; });
      await page.waitForTimeout(250);
      const idleCardRectReads = await page.evaluate(() => window.cardRectReads);
      if (floatingLetters && idleCardRectReads > 20) throw new Error('Idle animation keeps measuring historical cards: '+idleCardRectReads);
      await page.evaluate(() => {
        const shell=document.querySelector('.chat-messages-shell');
        shell.dispatchEvent(new WheelEvent('wheel',{deltaY:-1000,bubbles:true}));
        shell.scrollTop=shell.scrollHeight/2;
      });
      await page.waitForFunction(() => document.querySelector('.chat-scroll-to-bottom').classList.contains('is-visible'));
      const savedReading = await page.evaluate(() => ({top:document.querySelector('.chat-messages-shell').scrollTop,count:document.querySelectorAll('.chat-response-content').length}));
      errors.push(...await page.evaluate(() => window.__desktopTestErrors ?? []));
      await page.goto(fixtureURL);
      await page.waitForFunction(() => typeof window.configure === 'function');
      await page.evaluate(({size,key})=>{window.configure(size,key);window.openChat(key+'-b');},{size,key});
      await page.waitForFunction(saved => {
        const shell=document.querySelector('.chat-messages-shell');
        return shell && Math.abs(shell.scrollTop-saved.top)<2 && document.querySelectorAll('.chat-response-content').length===saved.count;
      },savedReading);
      interactionResults = {drafts:true,toolToggle:withTools,scrollAndStream:true,returnToBottom:true,editorReturn:true,rapidSwitches:true,historyPrepend,automaticHistory,reloadReadingPosition:true,tokenPacing,snapshotContinuity:floatingLetters,composerSides:floatingLetters,settledMarkdown:floatingLetters,inFlightMarkdown:floatingLetters,idleCardRectReads};
    }
    if (maxCachedMs && summary(true).p95Ms > maxCachedMs) budgetFailures.push(`${size} messages: cached p95 ${summary(true).p95Ms} ms exceeds ${maxCachedMs} ms`);
    if(minAnimationFPS && (!flightPerformance || flightPerformance.fps<minAnimationFPS)) budgetFailures.push(`${size} messages: animation ${flightPerformance?.fps??0} FPS is below ${minAnimationFPS}`);
    if(maxAnimationGapMs && (!flightPerformance || flightPerformance.frameGapMaxMs>maxAnimationGapMs)) budgetFailures.push(`${size} messages: animation maximum frame gap ${flightPerformance?.frameGapMaxMs??'unmeasured'} ms exceeds ${maxAnimationGapMs}`);
    cases.push({ messages: size, firstLoad: summary(false), cached: summary(true), interactions: interactionResults, flightPerformance, report });
    console.log(`${size} messages: first load median ${summary(false).medianMs} ms; cached median ${summary(true).medianMs} ms, p95 ${summary(true).p95Ms} ms`);
    if(flightPerformance) console.log(`Animation: ${flightPerformance.fps.toFixed(1)} FPS (idle ${flightPerformance.idleFPS.toFixed(1)}), p95 gap ${flightPerformance.frameGapP95Ms} ms, max gap ${flightPerformance.frameGapMaxMs} ms`);
  }
  errors.push(...await page.evaluate(() => window.__desktopTestErrors ?? []));
  if (errors.length) throw new Error(errors.join("\n"));
  const report = { schemaVersion: 1, benchmark: "production React build, synthetic Markdown/code/table histories, real ChatView + useChatRuntime", browser: browser.version(), engine, viewport: { width: 1280, height: 800 }, serverDelayMs, repeats, floatingLetters, withTools, interactions, maxCachedMs, minAnimationFPS, maxAnimationGapMs, animationSampleMs, budgetFailures, generatedAt: new Date().toISOString(), cases };
  await writeFile(output, JSON.stringify(report, null, 2));
  console.log(`Report: ${output}`);
  if (budgetFailures.length) throw new Error(budgetFailures.join("\n"));
} finally {
  if (browser) await browser.close();
  if (server) await new Promise(resolve => server.close(resolve));
  await rm(directory, { recursive: true, force: true });
}
