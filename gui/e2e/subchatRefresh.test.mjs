import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { chromium } from "playwright-core";

const fixtureSource = `
        import { createElement } from 'react';
        import { createRoot } from 'react-dom/client';
        import { SubchatNavigator } from '/src/chat/subagents/Navigator.tsx';
        import { liveChatFromPayload } from '/src/chat/chatClient.ts';
        import '/src/chat/chat.css';
        import '/src/pulse.css';
        const started = new Date(Date.now() - 20000).toISOString();
        const child = { id: 'child-call', name: 'subagent', subagentDepth: 2, status: 'success',
          result: { status: 'success', subchatId: 'child', subchatStatus: 'done' } };
        const root = { id: 'root-call', name: 'subagent', status: 'success',
          result: { status: 'success', subchatId: 'parent', subchatStatus: 'paused' } };
        window.childStatus = 'running';
        window.childText = 'Working on child';
        window.loads = { parent: 0, child: 0 };
        const app = createRoot(document.getElementById('root'));
        window.unmount = () => app.unmount();
        app.render(createElement(SubchatNavigator, { root, tools: [root], onCollapse() {},
          async loadSubchat(id) {
            window.loads[id]++;
            return liveChatFromPayload({ id, status: id === 'parent' ? 'paused' : window.childStatus,
              messages: [{ id: id + '-task', role: 'user', content: 'Inspect' },
                { id: id + '-reply', role: 'assistant', createdAt: started,
                  content: id === 'parent' ? 'Parent paused' : window.childText,
                  workedFor: 20, toolCalls: id === 'parent' ? [child] : [] }] }, 'project');
          }
        }));
`;

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, plugins: [{ name: "subchat-fixture", resolveId(id) { if (id === "/__subchat_fixture.js") return id; }, load(id) { if (id === "/__subchat_fixture.js") return fixtureSource; } }], server: { host: "127.0.0.1", port: 0, hmr: false }, esbuild: { jsx: "automatic" } });
await server.listen();
after(() => server.close());
const browser = await chromium.launch({ executablePath: process.env.SOLOMON_TEST_BROWSER || "/usr/bin/chromium", headless: true, args: ["--no-sandbox"] });
after(() => browser.close());

test("open nested subchats refresh their own status, transcript and live timer", async () => {
  const page = await browser.newPage();
  page.on("pageerror", error => console.error(error.message));
  try {
    await page.route("**/__subchat_fixture", route => route.fulfill({ contentType: "text/html", body: `
      <div id="root" class="app-shell"></div>
      <script type="module" src="/__subchat_fixture.js"></script>` }));
    await page.goto(`${server.resolvedUrls.local[0]}__subchat_fixture`);
    await page.getByRole("button", { name: "Open L2 Subagent chat", exact: true }).click({ timeout: 10000 });
    await page.waitForFunction(() => document.querySelector('.chat-subchat-panel.is-running .chat-worked-for.is-live'));
    const counter = page.locator(".chat-worked-for.is-live");
    const first = await counter.textContent();
    await page.waitForFunction(previous => document.querySelector('.chat-worked-for.is-live')?.textContent !== previous, first);
    assert.equal(await counter.locator(".chat-worked-for-wave").evaluate(node => getComputedStyle(node).animationName), "chat-worked-for-glow");
    await page.evaluate(() => { window.childText = 'New child output'; });
    await page.getByText("New child output", { exact: true }).waitFor();
    await page.evaluate(() => { window.childStatus = 'done'; });
    await page.waitForFunction(() => document.querySelector('.chat-subchat-panel.is-success') && !document.querySelector('.chat-worked-for.is-live'));
    await page.evaluate(() => { window.childStatus = 'running'; window.childText = 'Child resumed'; });
    await page.waitForFunction(() => document.querySelector('.chat-subchat-panel.is-running .chat-worked-for.is-live'));
    await page.getByText("Child resumed", { exact: true }).waitFor();
    await page.evaluate(() => window.unmount());
    const count = await page.evaluate(() => window.loads.child);
    await new Promise(resolve => setTimeout(resolve, 1200));
    assert.equal(await page.evaluate(() => window.loads.child), count);
  } finally {
    await page.close();
  }
});
