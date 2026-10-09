import assert from "node:assert/strict";
import { test } from "node:test";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false }, esbuild: { jsx: "automatic" } });
const { ChatMessageGroups } = await server.ssrLoadModule("/src/chat/ChatMessageGroups.tsx");

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

const { MessageFooter, WorkedForCounter } = await server.ssrLoadModule("/src/chat/ChatMessageFooter.tsx");

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
