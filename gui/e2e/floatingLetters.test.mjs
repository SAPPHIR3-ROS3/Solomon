import assert from "node:assert/strict";
import { after, test } from "node:test";
import { existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { chromium } from "playwright-core";

const fixture = `
import { useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { FloatingLetters } from '/src/chat/FloatingLetters.tsx';
import { MarkdownContent } from '/src/chat/MarkdownContent.tsx';
import { ChatSettings } from '/src/settings/ChatSettings.tsx';
import { useGUIChatPreferencesState } from '/src/settings/chatPreferences.ts';
function Fixture() {
  const viewRef = useRef(null);
  const { chat } = useGUIChatPreferencesState();
  const [content, update] = useState('Historical **response**');
  const [messageID, setMessageID] = useState('assistant-1');
  window.showResponse = (id, text) => { setMessageID(id); update(text); };
  const [working, setWorking] = useState(false);
  window.updateResponse = update;
  window.setWorking = setWorking;
  return <><ChatSettings query="" /><section ref={viewRef} style={{position:'relative',height:400,color:'#eee',background:'#151515'}}>
    {chat.floatingLetters && <FloatingLetters density={chat.floatingLettersDensity} content={content} working={working} messageID={messageID} viewRef={viewRef} />}
    <div className="chat-messages-shell" style={{height:400,overflow:'auto'}}>
      <article className="is-assistant" style={{padding:60}}><div className="chat-response-content" data-message-id={messageID}><MarkdownContent content={content}/></div></article>
    </div>
  </section><textarea aria-label="Composer" /></>;
}
createRoot(document.getElementById('root')).render(<Fixture />);
`;
const viewFixture = `
import { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { ChatView } from '/src/chat/ChatView.tsx';
import { setFloatingLetters } from '/src/settings/chatPreferences.ts';
import '/src/styles.css';
import '/src/home/welcome.css';
function Fixture() {
  const [content, update] = useState('');
  const [working, setWorking] = useState(true);
  window.setWorking = setWorking;
  window.disableLetters = () => setFloatingLetters(false);
  window.updateResponse = update;
  const chat = {id:'live',projectID:'fixture',title:'Letters',status:working?'running':'done',messages:[
    {id:'user',role:'user',content:'Write a response'},
    {id:'assistant',role:'assistant',content},
  ]};
  return <ChatView chat={chat} isStreaming={working} onSend={()=>{}} onDeleteMessage={()=>{}} onStopTool={()=>{}} onStopStreaming={()=>{}} />;
}
createRoot(document.getElementById('root')).render(<Fixture />);
`;
const snapshotFixture = `
import { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { ChatView } from '/src/chat/ChatView.tsx';
import { applyLiveStreamEvent } from '/src/chat/chatClient.ts';
import '/src/styles.css';
import '/src/home/welcome.css';
function Fixture() {
  const [chat, setChat] = useState(() => applyLiveStreamEvent({id:'snapshot',projectID:'fixture',title:'Snapshot',messages:[{id:'user',role:'user',content:'Test streaming'}]}, {type:'assistant_start',turn:1,checkpoint_seq:1}));
  window.streamEvent = event => setChat(current => applyLiveStreamEvent(current,event));
  return <ChatView chat={chat} isStreaming={chat.status==='running'} onSend={()=>{}} onDeleteMessage={()=>{}} onStopTool={()=>{}} onStopStreaming={()=>{}} />;
}
createRoot(document.getElementById('root')).render(<Fixture />);
`;
const server = await createServer({
  root: fileURLToPath(new URL("..", import.meta.url)), configFile: false,
  plugins: [{ name: "floating-fixture", resolveId: id => ["/__floating.tsx", "/__floating_view.tsx", "/__snapshot.tsx"].includes(id) ? id : undefined,
    load: id => id === "/__floating.tsx" ? fixture : id === "/__floating_view.tsx" ? viewFixture : id === "/__snapshot.tsx" ? snapshotFixture : undefined }],
  server: { host: "127.0.0.1", port: 0, hmr: false }, esbuild: { jsx: "automatic" },
});
await server.listen();
after(() => server.close());
const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/google-chrome"].find(existsSync);
assert.ok(executablePath, "Install Chromium or set SOLOMON_TEST_BROWSER");
const browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
after(() => browser.close());

test("floating letters persist, converge into streamed Markdown, and clean up", async () => {
  const page = await browser.newPage();
  page.setDefaultTimeout(10000);
  const errors = [];
  const patches = [];
  page.on("pageerror", error => errors.push(error.message));
  let chat = { autoCloseToolCalls: false, startToolCallsCollapsed: false, floatingLetters: false };
  await page.addInitScript(() => {
    const originalClear = CanvasRenderingContext2D.prototype.clearRect;
    const originalFill = CanvasRenderingContext2D.prototype.fillText;
    CanvasRenderingContext2D.prototype.clearRect = function (...args) {
      window.letterPaintCount = 0;
      return originalClear.apply(this, args);
    };
    CanvasRenderingContext2D.prototype.fillText = function (...args) {
      window.letterPaintCount = (window.letterPaintCount || 0) + 1;
      return originalFill.apply(this, args);
    };
  });
  await page.route("**/__solomon/gui-settings", async route => {
    if (route.request().method() === "PATCH") {
      const patch = route.request().postDataJSON();
      patches.push(patch);
      chat = { ...chat, ...patch.chat };
    }
    await route.fulfill({ json: { chat } });
  });
  await page.route("**/__floating", route => route.fulfill({ contentType: "text/html", body: '<div id="root"></div><script type="module" src="/__floating.tsx"></script>' }));
  const highlightSize = () => page.evaluate(() => Array.from(CSS.highlights.values()).reduce((n, value) => n + value.size, 0));
  try {
    await page.goto(`${server.resolvedUrls.local[0]}__floating`);
    const toggle = page.getByRole("switch", { name: "Floating letters", exact: true });
    await toggle.waitFor();
    assert.equal(await toggle.getAttribute("aria-checked"), "false");
    assert.equal(await page.locator("canvas").count(), 0);
    assert.equal(await page.getByRole("switch").last().getAttribute("aria-labelledby"), "floating-letters-label");
    assert.equal(await page.getByRole("group", { name: "Letter density" }).count(), 0);
    await toggle.click();
    await page.locator("canvas").waitFor();
    assert.deepEqual(patches, [{ chat: { floatingLetters: true } }]);
    assert.equal(await page.getByRole("radio", { name: "Medium", exact: true }).isChecked(), true);
    assert.equal(await highlightSize(), 0, "existing history must stay visible");
    for (const [label, count] of [["Low", 16], ["Medium", 32], ["High", 64]]) {
      await page.getByRole("radio", { name: label, exact: true }).click();
      await page.waitForFunction(value => document.querySelector(`input[name="floating-letters-density"][value="${value}"]`)?.checked, label.toLowerCase());
      await page.waitForFunction(expected => window.letterPaintCount === expected, count);
      assert.equal(await page.locator("canvas").getAttribute("data-density"), label.toLowerCase());
      assert.equal(chat.floatingLettersDensity, label.toLowerCase());
    }
    await page.evaluate(() => { window.setWorking(true); window.updateResponse('Historical **response** with new letters'); });
    await page.waitForFunction(() => Array.from(CSS.highlights.values()).some(value => value.size > 0));
    assert.equal(await page.locator(".chat-response-content strong").textContent(), "response");
    await page.getByRole("textbox", { name: "Composer" }).fill("still typing");
    await page.waitForFunction(() => Array.from(CSS.highlights.values()).every(value => value.size === 0));
    assert.equal(await page.locator(".chat-response-content").textContent(), "Historical response with new letters");
    assert.equal(await page.getByRole("textbox", { name: "Composer" }).inputValue(), "still typing");
    // A real canvas paint, not just a mounted empty layer.
    assert.ok(await page.locator("canvas").evaluate(canvas => canvas.getContext('2d').getImageData(0, 0, canvas.width, canvas.height).data.some((value, index) => index % 4 === 3 && value > 0)));
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.evaluate(() => window.updateResponse('Reduced motion **response**'));
    assert.equal(await page.locator("canvas").isVisible(), false);
    assert.equal(await highlightSize(), 0);
    await page.emulateMedia({ reducedMotion: "no-preference" });
    await page.evaluate(() => window.updateResponse('Another **response** arriving'));
    await page.waitForFunction(() => Array.from(CSS.highlights.values()).some(value => value.size > 0));
    await toggle.click();
    await page.locator("canvas").waitFor({ state: "detached" });
    assert.equal(await highlightSize(), 0, "disabling reveals all in-flight text");
    assert.equal(await page.getByRole("group", { name: "Letter density" }).count(), 0);
    assert.equal(await page.locator(".chat-response-content").textContent(), "Another response arriving");
    await toggle.click();
    await page.reload();
    await page.locator("canvas").waitFor();
    assert.equal(await toggle.getAttribute("aria-checked"), "true");
    assert.equal(await page.getByRole("radio", { name: "High", exact: true }).isChecked(), true);
    await page.waitForFunction(() => window.letterPaintCount === 64);
    assert.equal(await highlightSize(), 0);
    assert.deepEqual(errors, []);
  } finally { await page.close(); }
});


test("the real ChatView animates new assistant text while the composer stays usable", async () => {
  const page = await browser.newPage({ viewport: { width: 1200, height: 900 } });
  const errors = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.addInitScript(() => {
    const original = CanvasRenderingContext2D.prototype.fillText;
    CanvasRenderingContext2D.prototype.fillText = function (...args) {
      original.apply(this, args);
      if (!this.canvas.classList.contains('chat-floating-letters')) return;
      const composer = document.querySelector('.chat-composer .welcome-composer')?.getBoundingClientRect();
      if (!composer) return;
      const bounds = this.canvas.getBoundingClientRect();
      // Probe both the protected surface and the formerly clipped side band.
      original.call(this, 'M', composer.left + composer.width / 2 - bounds.left, composer.top + composer.height / 2 - bounds.top);
      original.call(this, 'M', (composer.right + Math.min(bounds.right, innerWidth)) / 2 - bounds.left, composer.top + composer.height / 2 - bounds.top);
    };
  });
  try {
    await page.route("**/__solomon/**", route => route.fulfill({ json: {
      projects: [], chat: { floatingLetters: true, floatingLettersDensity: "high", autoCloseToolCalls: false, startToolCallsCollapsed: false },
      current: { provider: "", model: "" }, providers: [], recent: [],
    } }));
    await page.route("**/__floating_view", route => route.fulfill({ contentType: "text/html", body: '<div id="root" class="app-shell"></div><script type="module" src="/__floating_view.tsx"></script>' }));
    await page.goto(`${server.resolvedUrls.local[0]}__floating_view`);
    await page.locator("canvas.chat-floating-letters").waitFor();
    assert.equal(await page.locator("canvas").getAttribute("data-density"), "high");
    await page.evaluate(() => window.updateResponse('The letters form a **real response**.'));
    await page.waitForFunction(() => Array.from(CSS.highlights.values()).some(value => value.size > 0));
    await page.getByRole("textbox", { name: "Message", exact: true }).fill("Next question");
    await page.waitForFunction(() => Array.from(CSS.highlights.values()).every(value => value.size === 0));
    assert.equal(await page.locator('.chat-message.is-assistant strong').textContent(), 'real response');
    assert.equal(await page.getByRole("textbox", { name: "Message", exact: true }).inputValue(), "Next question");
    assert.ok(await page.evaluate(() => {
      const layer = document.querySelector("canvas").getBoundingClientRect();
      const composer = document.querySelector(".chat-composer .welcome-composer").getBoundingClientRect();
      return layer.bottom >= composer.bottom;
    }), "floating letters cover the space beside the composer");
    await page.waitForFunction(() => {
      const canvas = document.querySelector('canvas');
      const bounds = canvas.getBoundingClientRect();
      const composer = document.querySelector('.chat-composer .welcome-composer').getBoundingClientRect();
      const ratio = canvas.width / bounds.width;
      const context = canvas.getContext('2d');
      const hasPaint = (left, top, width, height) => context.getImageData(Math.round((left-bounds.left)*ratio),Math.round((top-bounds.top)*ratio),Math.floor(width*ratio),Math.floor(height*ratio)).data.some((value,index)=>index%4===3 && value>0);
      return hasPaint(composer.right+2,composer.top+4,Math.min(bounds.right,innerWidth)-composer.right-4,composer.height-8)
        && !hasPaint(composer.left+30,composer.top+30,composer.width-60,composer.height-60);
    });
    assert.deepEqual(errors, []);
  } finally { await page.close(); }
});

test("user messages and tool cards fully occlude floating letters", async () => {
  const page = await browser.newPage();
  page.setDefaultTimeout(10000);
  const errors = [];
  page.on("pageerror", error => errors.push(error.message));
  try {
    await page.route("**/__solomon/gui-settings", route => route.fulfill({ json: {
      chat: { floatingLetters: true, floatingLettersDensity: "high", autoCloseToolCalls: false, startToolCallsCollapsed: false },
    } }));
    await page.route("**/__floating", route => route.fulfill({ contentType: "text/html", body: '<div id="root"></div><script type="module" src="/__floating.tsx"></script>' }));
    await page.goto(`${server.resolvedUrls.local[0]}__floating`);
    await page.locator("canvas").waitFor();
    const hasPaint = () => {
      const canvas = document.querySelector("canvas");
      return canvas.getContext("2d").getImageData(0, 0, canvas.width, canvas.height).data.some((value, index) => index % 4 === 3 && value > 0);
    };
    for (const className of ["chat-message is-user", "chat-tool-card", "chat-subagent-card"]) {
      await page.waitForFunction(hasPaint);
      await page.evaluate(className => {
        const canvas = document.querySelector("canvas");
        const shell = document.querySelector(".chat-messages-shell");
        const card = document.createElement("div");
        card.id = "occluding-card";
        card.className = className;
        const bounds = canvas.getBoundingClientRect();
        // An intentionally transparent card still blocks the decorative layer.
        card.style.cssText = `position:relative;width:100%;height:${bounds.height}px;margin-bottom:-${bounds.height}px;border-radius:0;background:transparent;pointer-events:none`;
        shell.prepend(card);
        window.setWorking(true);
        window.updateResponse(`New response beneath ${className}`);
      }, className);
      await page.waitForFunction(() => Array.from(CSS.highlights.values()).some(value => value.size > 0));
      await page.waitForFunction(() => {
        const canvas = document.querySelector("canvas");
        return !canvas.getContext("2d").getImageData(0, 0, canvas.width, canvas.height).data.some((value, index) => index % 4 === 3 && value > 0);
      });
      await page.evaluate(() => document.getElementById("occluding-card").remove());
      await page.waitForFunction(hasPaint);
    }
    assert.deepEqual(errors, []);
  } finally { await page.close(); }
});


test("fast bursts are presented in ordered small tokens even after the stream finishes", async () => {
  const page = await browser.newPage({ viewport: { width: 1200, height: 900 } });
  page.setDefaultTimeout(10000);
  const errors = [];
  page.on("pageerror", error => errors.push(error.message));
  let chat = { floatingLetters: true, autoCloseToolCalls: false, startToolCallsCollapsed: false };
  try {
    await page.route("**/__solomon/**", route => {
      if (route.request().method() === "PATCH") chat = { ...chat, ...route.request().postDataJSON().chat };
      return route.fulfill({ json: { projects: [], chat, current: { provider: "", model: "" }, providers: [], recent: [] } });
    });
    await page.route("**/__floating_view", route => route.fulfill({ contentType: "text/html", body: '<div id="root" class="app-shell"></div><script type="module" src="/__floating_view.tsx"></script>' }));
    await page.goto(`${server.resolvedUrls.local[0]}__floating_view`);
    await page.locator("canvas").waitFor();
    const burst = "abcdefghijklmnopqrstuvxyz0123456789".repeat(5);
    const full = burst + "FINALTOKENS0123456789";
    await page.evaluate(burst => {
      const response = document.querySelector('.chat-message.is-assistant .chat-response-content');
      window.presentedPrefixes = [];
      new MutationObserver(() => window.presentedPrefixes.push(response.textContent)).observe(response, { childList: true, characterData: true, subtree: true });
      window.updateResponse(burst);
    }, burst);
    await page.waitForFunction(() => document.querySelector('.chat-message.is-assistant .chat-response-content').textContent.length > 0);
    assert.ok((await page.locator('.chat-message.is-assistant .chat-response-content').textContent()).length < burst.length);
    await page.getByRole("textbox", { name: "Message", exact: true }).fill("typing during the queue");
    await page.evaluate(full => { window.updateResponse(full); window.setWorking(false); }, full);
    await page.waitForFunction(full => document.querySelector('.chat-message.is-assistant .chat-response-content').textContent === full, full);
    const prefixes = await page.evaluate(() => window.presentedPrefixes);
    assert.ok(prefixes.length > 20, "a fast burst must have many distinct visual steps");
    let previous = 0;
    for (const prefix of prefixes) {
      assert.ok(full.startsWith(prefix), "characters stay ordered and no token is skipped");
      assert.ok(prefix.length - previous <= 4, "a render must not reveal an entire line");
      previous = prefix.length;
    }
    assert.equal(await page.getByRole("textbox", { name: "Message", exact: true }).inputValue(), "typing during the queue");
    const reduced = full + "REDUCEDMOTION".repeat(20);
    await page.evaluate(text => { window.setWorking(true); window.updateResponse(text); }, reduced);
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.waitForFunction(text => document.querySelector('.chat-message.is-assistant .chat-response-content').textContent === text, reduced);
    await page.emulateMedia({ reducedMotion: "no-preference" });
    const disabled = reduced + "DISABLED".repeat(20);
    await page.evaluate(text => window.updateResponse(text), disabled);
    await page.evaluate(() => window.disableLetters());
    await page.locator("canvas").waitFor({state:"detached"});
    assert.equal(await page.locator('.chat-message.is-assistant .chat-response-content').textContent(), disabled);
    assert.deepEqual(errors, []);
  } finally { await page.close(); }
});


test("settled letters do not replay when Markdown closes or a response is revisited", async () => {
  const page = await browser.newPage();
  page.setDefaultTimeout(10000);
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  const hiddenLetters = () => page.evaluate(() => Array.from(CSS.highlights.values()).reduce((count, value) => count + value.size, 0));
  try {
    await page.route('**/__solomon/gui-settings', route => route.fulfill({json:{chat:{floatingLetters:true,autoCloseToolCalls:false,startToolCallsCollapsed:false}}}));
    await page.route('**/__floating', route => route.fulfill({contentType:'text/html',body:'<div id="root"></div><script type="module" src="/__floating.tsx"></script>'}));
    await page.goto(`${server.resolvedUrls.local[0]}__floating`);
    await page.locator('canvas').waitFor();
    await page.evaluate(()=>window.setWorking(true));
    for (const [open, closed, selector, text] of [
      ['**Settled letters', '**Settled letters**', 'strong', 'Settled letters'],
      ['Before `existing code', 'Before `existing code`', 'code', 'existing code'],
      ['[Existing label](https://example.com', '[Existing label](https://example.com)', 'a', 'Existing label'],
    ]) {
      await page.evaluate(text=>window.updateResponse(text),open);
      await page.waitForFunction(text=>document.querySelector('.chat-response-content').textContent===text,open);
      await page.waitForFunction(()=>Array.from(CSS.highlights.values()).every(value=>value.size===0));
      await page.evaluate(text=>window.updateResponse(text),closed);
      await page.locator(`.chat-response-content ${selector}`).waitFor();
      assert.equal(await page.locator(`.chat-response-content ${selector}`).textContent(),text);
      assert.equal(await hiddenLetters(),0,'closing Markdown must not send settled letters flying again');
    }
    await page.evaluate(()=>window.showResponse('second','Different response'));
    await page.waitForFunction(()=>document.querySelector('.chat-response-content').textContent==='Different response');
    await page.waitForFunction(()=>Array.from(CSS.highlights.values()).every(value=>value.size===0));
    await page.evaluate(()=>window.showResponse('assistant-1','[Existing label](https://example.com)'));
    await page.locator('.chat-response-content a').waitFor();
    assert.equal(await hiddenLetters(),0,'returning to a known response must retain its settled letters');
    assert.deepEqual(errors,[]);
  } finally { await page.close(); }
});

test("closing an inline delimiter keeps in-flight highlights bound to the formatted text", async () => {
  const page = await browser.newPage();
  page.setDefaultTimeout(10000);
  try {
    await page.route('**/__solomon/gui-settings', route => route.fulfill({json:{chat:{floatingLetters:true,autoCloseToolCalls:false,startToolCallsCollapsed:false}}}));
    await page.route('**/__floating', route => route.fulfill({contentType:'text/html',body:'<div id="root"></div><script type="module" src="/__floating.tsx"></script>'}));
    await page.goto(`${server.resolvedUrls.local[0]}__floating`);
    await page.locator('canvas').waitFor();
    await page.evaluate(()=>window.setWorking(true));
    for(const [delimiter,selector] of [['**','strong'],['*','em'],['__','strong'],['_','em'],['~~','del'],['`','code']]){
      await page.evaluate(({delimiter})=>window.showResponse('format-'+delimiter,'Prefix '+delimiter+'abcdefghijk'.repeat(4)),{delimiter});
      await page.waitForFunction(()=>Array.from(CSS.highlights.values()).some(value=>value.size>0));
      await page.waitForTimeout(550);
      await page.evaluate(({delimiter,selector})=>{
        const response=document.querySelector('.chat-response-content');
        window.formattedFlightSnapshot=null;
        const observer=new MutationObserver(()=>{
          if(!response.querySelector(selector)) return;
          const ranges=Array.from(CSS.highlights.values()).flatMap(value=>Array.from(value));
          window.formattedFlightSnapshot={count:ranges.length,invalid:ranges.filter(range=>!range.startContainer.isConnected || !range.toString()).length};
          observer.disconnect();
        });
        observer.observe(response,{childList:true,characterData:true,subtree:true});
        window.updateResponse('Prefix '+delimiter+'abcdefghijk'.repeat(4)+delimiter);
      },{delimiter,selector});
      await page.waitForFunction(()=>window.formattedFlightSnapshot!==null);
      const snapshot=await page.evaluate(()=>window.formattedFlightSnapshot);
      assert.ok(snapshot.count>0, 'the delimiter must close while letters are still travelling');
      assert.equal(snapshot.invalid,0,`${delimiter}: formatting must not briefly reveal the block before hiding it again`);
      await page.waitForFunction(()=>Array.from(CSS.highlights.values()).every(value=>value.size===0));
    }
  } finally { await page.close(); }
});

test("the persisted snapshot keeps the streaming response's progress and settled glyphs", async () => {
  const page = await browser.newPage();
  page.setDefaultTimeout(15000);
  const content = '## Test di streaming e formattazione\n\nQuesto paragrafo contiene **grassetto**, *corsivo* e `codice`. ' + 'Testo ancora in arrivo. '.repeat(8);
  try {
    await page.route('**/__solomon/**', route => route.fulfill({json:{chat:{floatingLetters:true},projects:[],providers:[],recent:[]}}));
    await page.route('**/__snapshot', route => route.fulfill({contentType:'text/html',body:'<div id="root" class="app-shell"></div><script type="module" src="/__snapshot.tsx"></script>'}));
    await page.goto(`${server.resolvedUrls.local[0]}__snapshot`);
    await page.locator('canvas').waitFor();
    await page.evaluate(content => window.streamEvent({type:'assistant_delta',channel:'content',delta:content}), content);
    await page.waitForFunction(() => document.querySelector('.is-assistant .chat-response-content')?.textContent.length > 90);
    await page.evaluate(content => {
      window.snapshotProgress = [];
      const read = () => document.querySelector('.is-assistant .chat-response-content')?.textContent ?? '';
      window.snapshotProgress.push(read());
      window.snapshotObserver = new MutationObserver(() => window.snapshotProgress.push(read()));
      window.snapshotObserver.observe(document.querySelector('.chat-messages-shell'),{childList:true,characterData:true,subtree:true});
      window.streamEvent({type:'chat_snapshot',chat:{id:'snapshot',title:'Snapshot',status:'done',messages:[{id:'saved-user',role:'user',content:'Test streaming'},{id:'saved-assistant',role:'assistant',checkpointSeq:1,content}]}});
    }, content);
    await page.waitForFunction(() => document.querySelector('[data-message-id="saved-assistant"]'));
    await page.waitForTimeout(300);
    const progress = await page.evaluate(() => window.snapshotProgress.map(text => text.length));
    for(let index=1;index<progress.length;index++) {
      assert.ok(progress[index]>=progress[index-1], `saving must not rewind the response: ${progress}`);
      assert.ok(progress[index]-progress[index-1]<=8, `saving must preserve token pacing: ${progress}`);
    }
    await page.waitForFunction(() => document.querySelector('.is-assistant .chat-response-content')?.textContent.endsWith('Testo ancora in arrivo.'));
    await page.waitForFunction(() => Array.from(CSS.highlights.values()).every(value=>value.size===0));
    await page.evaluate(content => {
      window.snapshotObserver.disconnect();
      window.streamEvent({type:'chat_snapshot',chat:{id:'snapshot',title:'Snapshot',status:'done',messages:[{id:'saved-user',role:'user',content:'Test streaming'},{id:'another-snapshot-id',role:'assistant',checkpointSeq:1,content}]}});
    },content);
    await page.locator('[data-message-id="another-snapshot-id"]').waitFor();
    assert.ok((await page.locator('.is-assistant .chat-response-content').textContent()).endsWith('Testo ancora in arrivo.'));
    assert.equal(await page.evaluate(()=>Array.from(CSS.highlights.values()).reduce((sum,value)=>sum+value.size,0)),0,'settled glyphs must not fly again after another snapshot');
    // A distinct checkpoint with the same text is a new response and must animate.
    await page.evaluate(content => {
      window.streamEvent({type:'assistant_start',turn:2,checkpoint_seq:2});
      window.streamEvent({type:'assistant_delta',channel:'content',delta:content});
    },content);
    await page.waitForFunction(()=>document.querySelectorAll('.is-assistant .chat-response-content').length===2);
    await page.waitForFunction(()=>Array.from(CSS.highlights.values()).some(value=>value.size>0));
    assert.ok((await page.locator('.is-assistant .chat-response-content').last().textContent()).length<content.length);
  } finally { await page.close(); }
});

test("in-flight motion does not repeatedly measure text or rebuild unchanged highlights", async () => {
  const page = await browser.newPage();
  try {
    await page.route('**/__solomon/gui-settings', route => route.fulfill({json:{chat:{floatingLetters:true}}}));
    await page.route('**/__floating', route => route.fulfill({contentType:'text/html',body:'<!doctype html><div id="root"></div><script type="module" src="/__floating.tsx"></script>'}));
    await page.goto(`${server.resolvedUrls.local[0]}__floating`);
    await page.locator('canvas').waitFor();
    await page.evaluate(()=>{window.setWorking(true);window.showResponse('motion','MovingLetters'.repeat(6));});
    await page.waitForFunction(()=>Array.from(CSS.highlights.values()).some(value=>value.size>0));
    await page.waitForTimeout(180);
    await page.evaluate(()=>{
      window.motionProbe={reads:0,highlightRebuilds:0,frames:0};
      const rect=Range.prototype.getBoundingClientRect;
      Range.prototype.getBoundingClientRect=function(...args){window.motionProbe.reads++;return rect.apply(this,args);};
      for(const value of CSS.highlights.values()) for(const method of ['clear','add']){
        const original=value[method];
        value[method]=function(...args){window.motionProbe.highlightRebuilds++;return original.apply(this,args);};
      }
      const count=()=>{window.motionProbe.frames++;if(window.motionProbe.frames<10)requestAnimationFrame(count);};
      requestAnimationFrame(count);
    });
    await page.waitForFunction(()=>window.motionProbe.frames>=8);
    const probe=await page.evaluate(()=>window.motionProbe);
    assert.equal(probe.reads,0,'unchanged flight destinations must be cached between layout changes');
    assert.equal(probe.highlightRebuilds,0,'unchanged hidden ranges must remain registered between frames');
    assert.ok(await page.evaluate(()=>Array.from(CSS.highlights.values()).some(value=>value.size>0)),'measure while letters are still in flight');
  } finally { await page.close(); }
});
