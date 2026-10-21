import assert from "node:assert/strict";
import { test } from "node:test";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false }, esbuild: { jsx: "automatic" } });
const { ChatMessageGroups } = await server.ssrLoadModule("/src/chat/ChatMessageGroups.tsx");
const { liveWorkedForSeconds, indexChatMessages } = await server.ssrLoadModule("/src/chat/chatMessageUtils.ts");

test("streaming retains historical entries and recomputes checkpoints when their context changes", () => {
  const user={id:'u',role:'user',content:'Question'};
  const answer={id:'a',role:'assistant',content:'Answer',toolCalls:[{id:'t',name:'shell'}]};
  const first=indexChatMessages([user,answer]);
  const updated=indexChatMessages([user,answer,{id:'next',role:'assistant',content:'New token'}]);
  assert.equal(first[0],updated[0]);
  assert.equal(first[1],updated[1]);
  const prefixed=[{id:'prefix',role:'user',content:'Previous question'},user,answer];
  const shifted=indexChatMessages(prefixed);
  assert.notEqual(shifted[2],first[1]);
  assert.deepEqual(shifted,indexChatMessages(prefixed.map(message=>({...message}))));
  assert.deepEqual(indexChatMessages([answer]),indexChatMessages([{...answer}]));
});

test("a pending user turn does not attach its live timer to the previous answer", () => {
  const messages = [
    {id:'old-user',role:'user',content:'Earlier question'},
    {id:'old-answer',role:'assistant',content:'Earlier answer',workedFor:14,workStartedAt:1000},
    {id:'new-user',role:'user',content:'Next question'},
  ];
  const chat = {id:'chat',title:'Chat',status:'running',runStartedAt:20000,messages};
  assert.equal(liveWorkedForSeconds(chat,25000),undefined);
  // Defend the renderer too, even if its caller passes a pending run timer.
  const waiting = renderToStaticMarkup(createElement(ChatMessageGroups,{messages:indexChatMessages(messages),isWorking:true,liveWorkedFor:5}));
  assert.doesNotMatch(waiting,/chat-worked-for is-live/);
  assert.match(waiting,/worked for 14s/);
  const running = {...chat,messages:[...messages,{id:'new-answer',role:'assistant',content:'',workStartedAt:22000}]};
  assert.equal(liveWorkedForSeconds(running,25000),3);
  const active = renderToStaticMarkup(createElement(ChatMessageGroups,{messages:indexChatMessages(running.messages),isWorking:true,liveWorkedFor:3}));
  assert.equal((active.match(/chat-worked-for is-live/g)??[]).length,1);
  assert.ok(active.indexOf('chat-worked-for is-live')>active.indexOf('Next question'));
});

test("groups intermediate assistant replies into expanded tool activity by default", () => {
  const tool = (id, checkpointSeq) => ({
    checkpointSeq,
    id,
    input: "command",
    intent: "run command",
    name: "shell",
    result: { output: "done", status: "success" },
    status: "success",
  });
  const indexed = (id, sequence, content, reasoning, toolCalls = []) => ({
    checkpoint: { branch: "", label: "[#" + String(sequence).padStart(3, "0") + "]", sequence },
    index: sequence,
    message: { content, id, reasoning, role: "assistant", toolCalls },
    toolMessageIDs: new Map(),
  });
  const messages = [
    indexed("a1", 0, "", "first thought", [tool("t1", 1)]),
    indexed("a2", 2, "intermediate response", "second thought"),
    indexed("a3", 3, "", "third thought", [tool("t2", 4)]),
    indexed("a4", 5, "final response", "final thought"),
  ];
  const html = renderToStaticMarkup(createElement(ChatMessageGroups, { messages, onOpenSubagent: () => {}, onStopTool: () => {} }));
  assert.equal(html.includes("intermediate response"), true);
  assert.equal(html.includes("final response"), true);
  assert.equal((html.match(/chat-tool-activity-controls/g) ?? []).length, 1);
  assert.equal((html.match(/chat-tool-card/g) ?? []).length, 2);
});

const { MessageFooter, WorkedForCounter, ReasoningBlock } = await server.ssrLoadModule("/src/chat/ChatMessageFooter.tsx");

test("reasoning controls require actual text rather than first-token timing", () => {
  for (const reasoning of [undefined, '', ' \n\t']) {
    const message = {id:'no-reasoning',role:'assistant',content:'Answer',reasoning,thoughtFor:4,stats:{ttftSeconds:4}};
    for (const isCollapsed of [false,true]) {
      const html = renderToStaticMarkup(createElement(ReasoningBlock,{message,isCollapsed,onToggle:()=>{}}));
      assert.match(html,/thought briefly/);
      assert.doesNotMatch(html,/role="button"|tabindex|aria-expanded|thought for/);
    }
    const html = renderToStaticMarkup(createElement(ChatMessageGroups,{messages:indexChatMessages([message])}));
    assert.match(html,/thought briefly/);
    assert.doesNotMatch(html,/Model reasoning|thought for/);
  }
  for (const thoughtFor of [undefined,3]) {
    const message = {id:'reasoning',role:'assistant',content:'Answer',reasoning:'Reasoning text',thoughtFor};
    const html = renderToStaticMarkup(createElement(ChatMessageGroups,{messages:indexChatMessages([message])}));
    assert.match(html,/chat-reasoning-copy/);
    assert.match(html,/Reasoning text/);
  }
});

test("il contatore illumina le lettere in onda durante il lavoro e diventa giallo alla fine", async () => {
  const active = renderToStaticMarkup(createElement(WorkedForCounter, { isLive: true, seconds: 12 }));
  const completed = renderToStaticMarkup(createElement(WorkedForCounter, { isLive: false, seconds: 12 }));
  assert.match(active, /class="chat-worked-for is-live"/);
  assert.match(completed, /class="chat-worked-for"/);
  assert.doesNotMatch(completed, /is-live/);
  assert.equal((active.match(/class="chat-worked-for-wave"/g) ?? []).length, 1);
  assert.doesNotMatch(active, /chat-worked-for-letter|animation-delay/);
  assert.match(active, /aria-label="worked for 12s"/);
  assert.match(active, /aria-hidden="true" class="chat-worked-for-wave"/);
  assert.doesNotMatch(completed, /chat-worked-for-letter/);
  assert.match(completed, /worked for 12s/);
  const css = await readFile(new URL("../src/chat/chat.css", import.meta.url), "utf8");
  const pulse = await readFile(new URL("../src/pulse.css", import.meta.url), "utf8");
  assert.match(css, /\.chat-worked-for\s*\{[^}]*color: var\(--color-crown-gold\)/);
  assert.match(css, /\.chat-worked-for\.is-live\s*\{[^}]*color: var\(--color-text-muted\)/);
  assert.match(pulse, /\.app-shell \.chat-worked-for\.is-live \.chat-worked-for-wave(?:\s*,[^{}]+)?\s*\{[^}]*animation: chat-worked-for-glow/);
  assert.doesNotMatch(pulse, /\.chat-worked-for\.is-live\s*\{\s*animation:/);
  assert.match(pulse, /from \{ background-position: -3ch 0; \}/);
  assert.match(pulse, /to \{ background-position: calc\(100% \+ 3ch\) 0; \}/);
  assert.match(pulse, /background-repeat: no-repeat;/);
  assert.match(pulse, /background-size: 3ch 100%;/);
  assert.match(pulse, /background-clip: text;/);
  assert.match(pulse, /@media \(prefers-reduced-motion: reduce\)\s*\{\s*\.app-shell \.chat-worked-for\.is-live \.chat-worked-for-wave(?:\s*,[^{}]+)?\s*\{\s*animation: none;/);
});

test("il footer mostra copia, dati e ora in questo ordine", () => {
  for (const role of ["assistant", "user"]) {
    const message = { id: "footer-order", role, content: "test", createdAt: 1700000000000, stats: {} };
    const html = renderToStaticMarkup(createElement(MessageFooter, { index: 0, message, onRequestDelete: role === "user" ? () => {} : undefined }));
    const copy = html.indexOf('class="chat-copy-message"');
    const stats = html.indexOf('class="chat-stats-control"');
    const time = html.indexOf("<time");
    const remove = html.indexOf('class="chat-delete-message"');
    assert.ok(copy >= 0 && time > copy);
    if (role === "assistant") {
      assert.ok(stats > copy && stats < time);
      assert.equal(remove, -1);
    } else {
      assert.equal(stats, -1);
      assert.ok(remove >= 0 && remove < copy);
    }
  }
});

test("il footer e il popover dei dati sono allineati a destra", async () => {
  const css = await readFile(new URL("../src/chat/chat.css", import.meta.url), "utf8");
  assert.match(css, /\.chat-message-footer\s*\{[^}]*justify-content: flex-end;/);
  assert.match(css, /\.chat-stats-popover\s*\{[^}]*right: 0;/);
});

await server.close();

test("completed work duration is visible on every assistant turn", () => {
 const messages = [
  {id:"u1", role:"user", content:"First"},
  {id:"a1", role:"assistant", content:"First answer", workedFor:14},
  {id:"u2", role:"user", content:"Second"},
  {id:"a2", role:"assistant", content:"Second answer", workedFor:27},
 ];
 const html = renderToStaticMarkup(createElement(ChatMessageGroups, {messages:indexChatMessages(messages)}));
 assert.match(html, /worked for 14s/);
 assert.match(html, /worked for 27s/);
});
