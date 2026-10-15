import assert from "node:assert/strict";
import { after, test } from "node:test";
import { existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { chromium } from "playwright-core";

const fixtureModule = `
import React from 'react';
import { createRoot } from 'react-dom/client';
import { useChatRuntime } from '/src/chat/useChatRuntime.ts';
import { saveChat, updateChat } from '/src/chat/chatStore.ts';
import { markNavigationCommit, navigationForChat, navigationReport } from '/src/chat/navigationPerformance.ts';
import { NavigationPerformanceSettings } from '/src/settings/NavigationPerformanceSettings.tsx';
const project = { id: 'project', name: 'Project', path: '/tmp/project' };
window.pending = [];
const originalFetch = window.fetch;
window.fetch = (url, options) => {
  if (!String(url).includes('/__solomon/')) return originalFetch(url, options);
  // Deliberately allow late replies after abort to exercise the request guard.
  return new Promise(resolve => window.pending.push({ path: new URL(url, location.href).pathname, resolve }));
};
window.reply = (suffix, payload, status = 200) => {
  const index = window.pending.findIndex(request => request.path.endsWith(suffix));
  if (index < 0) throw new Error('No pending request: ' + suffix);
  const [request] = window.pending.splice(index, 1);
  request.resolve(new Response(JSON.stringify(payload), { status, headers: { 'Content-Type': 'application/json' } }));
};
window.seed = (id, content) => saveChat({ id, title: id, projectID: project.id, source: 'daemon', status: 'success', messages: [{ id: 'message', role: 'assistant', content }] });
window.change = (id, content) => updateChat(id, chat => ({ ...chat, messages: chat.messages.map(message => ({ ...message, content })) }));
function Fixture() {
  const runtime = useChatRuntime();
  const [showPanel, setShowPanel] = React.useState(false);
  window.showPanel = () => setShowPanel(true);
  window.metrics = navigationReport;
  const navigationID = runtime.selectedChat ? navigationForChat(runtime.selectedChat.projectID, runtime.selectedChat.id) : undefined;
  React.useLayoutEffect(() => markNavigationCommit(navigationID, runtime.selectedChat?.messages.length ?? 0, 0), [navigationID]);
  window.openChat = id => { void runtime.openProjectChat(project, id); };
  window.clearChat = runtime.clearSelection;
  window.navigationState = { id: runtime.selectedChatID, loading: runtime.isLoading, error: runtime.error, branch: runtime.selectedChat?.branch };
  return <><section id="transcript">{runtime.selectedChat?.messages.map(message => <p key={message.id}>{message.content}</p>)}</section>{showPanel ? <NavigationPerformanceSettings query="" /> : null}</>;
}
createRoot(document.getElementById('root')).render(<Fixture />);
`;
const server = await createServer({
  root: fileURLToPath(new URL("..", import.meta.url)), configFile: false,
  plugins: [{ name: "navigation-fixture", resolveId: id => id === "/__navigation_fixture.tsx" ? id : undefined,
    load: id => id === "/__navigation_fixture.tsx" ? fixtureModule : undefined }],
  server: { host: "127.0.0.1", port: 0, hmr: false }, esbuild: { jsx: "automatic" },
});
await server.listen();
after(() => server.close());
const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/google-chrome"].find(existsSync);
assert.ok(executablePath, "Install Chromium or set SOLOMON_TEST_BROWSER");
const browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
after(() => browser.close());

async function fixture(t) {
  const page = await browser.newPage();
  page.setDefaultTimeout(10000);
  const errors = [];
  page.on("pageerror", error => errors.push(error.message));
  t.after(async () => { await page.close(); assert.deepEqual(errors, []); });
  await page.route("**/__navigation_fixture", route => route.fulfill({ contentType: "text/html", body: '<div id="root"></div><script type="module" src="/__navigation_fixture.tsx"></script>' }));
  await page.goto(`${server.resolvedUrls.local[0]}__navigation_fixture`);
  await page.waitForFunction(() => Boolean(window.openChat));
  return page;
}
const snapshot = (id, content) => ({ id, title: id, status: "done", mode: "agent", messages: [{ id: "message", role: "assistant", content }] });
async function waitForRequest(page, suffix) {
  await page.waitForFunction(suffix => window.pending.some(request => request.path.endsWith(suffix)), suffix);
}
async function reply(page, suffix, payload, status = 200) {
  await waitForRequest(page, suffix);
  await page.evaluate(({ suffix, payload, status }) => window.reply(suffix, payload, status), { suffix, payload, status });
}

test("cached threads display before the refresh and remain visible when refresh fails", async t => {
  const page = await fixture(t);
  await page.evaluate(() => { window.seed('a', 'cached A'); window.seed('b', 'cached B'); window.openChat('a'); });
  await page.getByText("cached A", { exact: true }).waitFor();
  assert.equal(await page.evaluate(() => window.navigationState.loading), false);
  await page.evaluate(() => window.openChat('b'));
  await page.getByText("cached B", { exact: true }).waitFor();
  assert.equal(await page.evaluate(() => window.navigationState.loading), false);
  await reply(page, "/chats/b", { error: "Refresh unavailable" }, 503);
  await page.waitForFunction(() => Boolean(window.navigationState.error));
  assert.equal(await page.locator("#transcript").textContent(), "cached B");
});

test("uncached threads display before Git metadata and cached threads refresh in place", async t => {
  const page = await fixture(t);
  await page.evaluate(() => window.openChat('new'));
  await page.waitForFunction(() => window.navigationState.loading);
  await reply(page, "/chats/new", snapshot("new", "fresh transcript"));
  await page.getByText("fresh transcript", { exact: true }).waitFor();
  assert.equal(await page.evaluate(() => window.navigationState.loading), false);
  await reply(page, "/branches", { isRepo: true, current: "main", branches: ["main"] });
  await reply(page, "/worktrees", { worktrees: [] });
  await page.waitForFunction(() => window.navigationState.branch === 'main');
  await page.evaluate(() => { window.originalTranscript = document.querySelector('#transcript'); window.openChat('new'); });
  await reply(page, "/chats/new", snapshot("new", "updated transcript"));
  await page.getByText("updated transcript", { exact: true }).waitFor();
  assert.equal(await page.evaluate(() => document.querySelector('#transcript') === window.originalTranscript), true);
});

test("late snapshots cannot reopen previous threads or a cleared selection", async t => {
  const page = await fixture(t);
  await page.evaluate(() => { window.openChat('a'); window.openChat('b'); });
  await reply(page, "/chats/b", snapshot("b", "selected B"));
  await page.getByText("selected B", { exact: true }).waitFor();
  await reply(page, "/chats/a", snapshot("a", "late A"));
  await page.evaluate(() => new Promise(resolve => setTimeout(resolve, 50)));
  assert.equal(await page.locator("#transcript").textContent(), "selected B");
  await page.evaluate(() => { window.openChat('c'); window.clearChat(); });
  await reply(page, "/chats/c", snapshot("c", "late C"));
  await page.evaluate(() => new Promise(resolve => setTimeout(resolve, 50)));
  assert.equal(await page.evaluate(() => window.navigationState.id), null);
  assert.equal(await page.locator("#transcript").textContent(), "");
});

test("refresh and Git metadata preserve updates received while the request was pending", async t => {
  const page = await fixture(t);
  await page.evaluate(() => { window.seed('a', 'original'); window.openChat('a'); });
  await waitForRequest(page, "/chats/a");
  await page.evaluate(() => window.change('a', 'new stream content'));
  await reply(page, "/chats/a", snapshot("a", "stale snapshot"));
  await reply(page, "/branches", { isRepo: true, current: "main", branches: ["main"] });
  await page.evaluate(() => window.change('a', 'latest stream content'));
  await reply(page, "/worktrees", { worktrees: [] });
  await page.waitForFunction(() => window.navigationState.branch === 'main');
  assert.equal(await page.locator("#transcript").textContent(), "latest stream content");
});

test("diagnostics record paint, request phases and browser stalls, survive reload and can be copied or cleared", async t => {
  const page = await fixture(t);
  await page.evaluate(() => { window.seed('metric', 'private message text'); window.openChat('metric'); });
  await page.waitForFunction(() => window.metrics().samples.at(-1)?.paintOpportunityMs !== undefined);
  await page.evaluate(() => new Promise(resolve => setTimeout(() => {
    const end = performance.now() + 100; while (performance.now() < end) {}
    resolve();
  }, 0)));
  await reply(page, "/chats/metric", snapshot("metric", "private updated text"));
  await reply(page, "/branches", { isRepo: true, current: "main", branches: ["main"] });
  await reply(page, "/worktrees", { worktrees: [] });
  await page.waitForFunction(() => window.metrics().samples.at(-1).spans.some(span => span.phase === 'git.worktrees'));
  const report = await page.evaluate(() => window.metrics());
  const sample = report.samples.at(-1);
  assert.equal(sample.cached, true);
  assert.equal(sample.outcome, "ready");
  assert.ok(sample.paintOpportunityMs >= sample.commitMs);
  for (const phase of ['chat.endpoint', 'chat.headers', 'chat.body', 'chat.json', 'chat.normalize', 'chat.total', 'cache.save', 'git.branches', 'git.worktrees']) {
    assert.ok(sample.spans.some(span => span.phase === phase), `missing ${phase}`);
  }
  assert.ok(!JSON.stringify(report).includes('private message'));
  if (report.longTasksSupported) await page.waitForFunction(() => window.metrics().samples.at(-1).longTasks.some(task => task.durationMs >= 90));
  await page.waitForFunction(() => {
    const samples = JSON.parse(localStorage.getItem('solomon.chat-navigation-performance.v1') ?? '[]');
    return samples.at(-1)?.spans.some(span => span.phase === 'git.worktrees');
  });
  await page.reload();
  await page.waitForFunction(() => Boolean(window.metrics));
  assert.equal(await page.evaluate(() => window.metrics().samples.at(-1).id), sample.id);
  await page.evaluate(() => { window.runtime = { ClipboardSetText: async text => { window.copiedReport = text; return true; } }; window.showPanel(); });
  await page.getByRole('button', { name: 'Copy report', exact: true }).click();
  await page.waitForFunction(() => Boolean(window.copiedReport));
  assert.equal(await page.evaluate(() => JSON.parse(window.copiedReport).samples.at(-1).id), sample.id);
  await page.getByRole('button', { name: 'Clear measurements', exact: true }).click();
  assert.equal(await page.evaluate(() => window.metrics().samples.length), 0);
});
