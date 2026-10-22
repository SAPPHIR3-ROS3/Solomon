import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false }, esbuild: { jsx: "automatic" } });
after(() => server.close());
const { fileChangeTree } = await server.ssrLoadModule("/src/chat/fileChangeTree.ts");
const { FileChangesCard } = await server.ssrLoadModule("/src/chat/FileChangesCard.tsx");
const { ChatMessageGroups } = await server.ssrLoadModule("/src/chat/ChatMessageGroups.tsx");
const { indexChatMessages } = await server.ssrLoadModule("/src/chat/chatMessageUtils.ts");
const { liveChatFromPayload, applyLiveStreamEvent } = await server.ssrLoadModule("/src/chat/chatClient.ts");
const { ChatSettings } = await server.ssrLoadModule("/src/settings/ChatSettings.tsx");
const { GeneralSettings } = await server.ssrLoadModule("/src/settings/GeneralSettings.tsx");
const changes = [{path:"docs/architecture/a.md",added:12,removed:1},{path:"gui/src/a.ts",added:23,removed:6},{path:"gui/src/b.ts",added:2,removed:0}];

test("tree compacts folder chains and aggregates only supplied chat changes", () => {
 const nodes=fileChangeTree(changes);
 assert.equal(nodes[0].name,"docs/architecture");
 assert.equal(nodes[1].name,"gui/src");
 assert.equal(nodes[1].added,25);assert.equal(nodes[1].removed,6);assert.equal(nodes[1].files,2);
 assert.equal(nodes[1].children.length,2);
 assert.deepEqual(fileChangeTree([]),[]);
});
test("collapsed card shows only the changed-files summary row", () => {
 const html=renderToStaticMarkup(createElement(FileChangesCard,{changes}));
 assert.match(html,/3 changed files/);assert.match(html,/\+37/);assert.match(html,/-7/);assert.doesNotMatch(html,/Open diff|Expand all folders|Collapse all folders/);
 assert.match(html,/aria-expanded="false"/);assert.doesNotMatch(html,/class="file-changes-tree"|docs\/architecture\/a\.md|gui\/src\/a\.ts/);
 assert.equal(renderToStaticMarkup(createElement(FileChangesCard,{changes:[]})),"");
});
test("completed editing turns show their own saved summary under the answer", () => {
 const messages=indexChatMessages([{id:"u",role:"user",content:"Edit"},{id:"a",role:"assistant",content:"Completed answer",fileChanges:changes},{id:"u2",role:"user",content:"Explain"},{id:"a2",role:"assistant",content:"Explanation"}]);
 const html=renderToStaticMarkup(createElement(ChatMessageGroups,{messages,isWorking:false}));
 assert.equal((html.match(/class="file-changes-card(?: [^"]*)?"/g)||[]).length,1);
 assert.ok(html.indexOf("Completed answer")<html.indexOf("file-changes-card"));
 assert.ok(html.indexOf("file-changes-card")<html.indexOf("Explanation"));
 const working=renderToStaticMarkup(createElement(ChatMessageGroups,{messages:messages.slice(0,2),isWorking:true}));
 assert.doesNotMatch(working,/class="file-changes-card(?: [^"]*)?"/);
});
test("final stream snapshots preserve file changes without copying them to another chat", () => {
 const payload={id:"chat-a",messages:[{id:"a",role:"assistant",content:"Done",fileChanges:changes}]};
 const chat=liveChatFromPayload(payload,"project");assert.deepEqual(chat.messages[0].fileChanges.map(c=>c.path),changes.map(c=>c.path));
 const final=applyLiveStreamEvent({...chat,status:"running"},{type:"chat_snapshot",chat:payload});
 assert.equal(final.messages[0].fileChanges[1].added,23);
 const other=liveChatFromPayload({id:"chat-b",messages:[{id:"b",role:"assistant",content:"Other"}]},"project");
 assert.equal(other.messages[0].fileChanges,undefined);
});
test("the enabled-by-default switch belongs to Chat and applies to both interfaces", () => {
 const chat=renderToStaticMarkup(createElement(ChatSettings,{query:"File changes"}));
 assert.match(chat,/File changes/);assert.match(chat,/aria-checked="true"/);assert.match(chat,/terminal/);
 const general=renderToStaticMarkup(createElement(GeneralSettings,{query:"File changes"}));
 assert.doesNotMatch(general,/File changes|Terminal file changes/);
});

test("an editing turn that cancels all pending changes hides the summary", () => {
 const chat=liveChatFromPayload({id:"chat-a",messages:[{id:"a",role:"assistant",content:"Removed",fileChangesRecorded:true}]},"project");
 assert.deepEqual(chat.messages[0].fileChanges,[]);
 const html=renderToStaticMarkup(createElement(ChatMessageGroups,{messages:indexChatMessages(chat.messages),isWorking:false}));
 assert.doesNotMatch(html,/file-changes-card|0 changed files|No pending file changes/);
});
