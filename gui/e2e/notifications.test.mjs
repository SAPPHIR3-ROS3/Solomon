import assert from "node:assert/strict";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import react from "@vitejs/plugin-react";
import { chromium } from "playwright-core";

test("composer history scrolls and new notifications expire after ten seconds", async () => {
  const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, plugins: [react()], server: { host: "127.0.0.1", port: 0 } });
  await server.listen();
  const browser = await chromium.launch({ executablePath: "/usr/bin/chromium", headless: true, args: ["--no-sandbox"] });
  try {
    const page = await browser.newPage({ permissions: ["clipboard-read", "clipboard-write"] });
    page.setDefaultTimeout(5000);
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const commandRequests = [];
    let notifications = Array.from({ length: 12 }, (_, index) => ({ id: String(index), createdAt: new Date(Date.now() - index * 60000).toISOString(), level: index === 0 ? "error" : "system", message: `Historical notice ${index}` }));
    await page.route("**/__solomon/**", async (route) => {
      const url = route.request().url();
      if (route.request().method() === "POST") commandRequests.push({ path: new URL(url).pathname, body: route.request().postDataJSON() });
      const data = url.endsWith("/notifications") ? notifications : url.includes("models") ? { current: { provider: "", model: "" }, providers: [], recent: [] } : { projects: [], reasoningEffort: "none", fastMode: false };
      await route.fulfill({ json: data });
    });
    await page.route("**/notification-fixture", (route) => route.fulfill({ contentType: "text/html", body: `<html><body><div id="root"></div><script type="module">
      import RefreshRuntime from '/@react-refresh';
      RefreshRuntime.injectIntoGlobalHook(window);
      window.$RefreshReg$ = () => {};
      window.$RefreshSig$ = () => (type) => type;
      window.__vite_plugin_react_preamble_installed__ = true;
    </script><script type="module">
      import React from '/node_modules/.vite/deps/react.js';
      import ReactDOM from '/node_modules/.vite/deps/react-dom_client.js';
      const { createRoot } = ReactDOM;
      import { NotificationToasts } from '/src/notifications/Notifications.tsx';
      import { watchNotifications } from '/src/notifications/store.ts';
      import '/src/styles.css';
      import '/src/home/welcome.css';
      import '/src/home/welcome-reasoning.css';
      import '/src/theme/themes.css';
      import '/src/typography.css';
      import '/src/home/welcome-branch.css';
      import '/src/responsive.css';
      import { ChatView } from '/src/chat/ChatView.tsx';
      import { useChatRuntime } from '/src/chat/useChatRuntime.ts';
      function CommandHarness() {
        const runtime = useChatRuntime();
        window.runHomeCommand = (content) => runtime.sendNewProjectMessage({id:'project-fixture',name:'Folder',path:'/tmp/fixture',chats:[],chatCount:0}, content);
        window.selectedCommandChat = runtime.selectedChatID;
        return null;
      }
      createRoot(document.getElementById('root')).render(React.createElement('main', {className:'app-shell', style:{'--right-panel-width':'240px','--top-bar-height':'56px'}}, React.createElement(ChatView, {chat:{id:'fixture', title:'Notifications', messages:[]}, workspaceName:'Folder', branch:'main', worktree:'Worktree', onSend:()=>{}, onStopStreaming:()=>{}}), React.createElement(NotificationToasts), React.createElement(CommandHarness))); 
      watchNotifications();
    </script></body></html>` }));
    await page.goto(server.resolvedUrls.local[0] + "notification-fixture");
    await page.waitForTimeout(1500);
    assert.deepEqual(errors, []);
    await page.evaluate(() => window.runHomeCommand("/help"));
    assert.deepEqual(commandRequests, [{path:"/__solomon/projects/project-fixture/commands", body:{content:"/help"}}]);
    assert.equal(await page.evaluate(() => window.selectedCommandChat), null);
    assert.equal(await page.locator("form .notification-bell").count(), 0);
    const bell = page.locator(".welcome-git-controls .notification-bell");
    const bellBounds = await bell.boundingBox();
    const composerBounds = await page.locator("form").boundingBox();
    assert.ok(bellBounds.y + bellBounds.height <= 720 && bellBounds.x + bellBounds.width <= 1280, JSON.stringify({bellBounds, composerBounds}));
    assert.ok(bellBounds.y >= composerBounds.y + composerBounds.height, "bell belongs below the textbox");
    await page.getByRole("button", { name: "Notifications", exact: true }).click();
    await page.getByRole("dialog").getByText("Historical notice 11", { exact: true }).waitFor();
    assert.equal(await page.locator(".notification-entry").count(), 12);
    assert.equal(await page.locator(".notification-toast").count(), 0);
    assert.ok(await page.locator(".notification-history").evaluate((node) => node.scrollHeight > node.clientHeight));
    await page.locator(".notification-entry").first().getByRole("button", { name: "Copy notification text", exact: true }).click();
    await page.locator(".notification-entry").first().getByRole("button", { name: "Notification copied", exact: true }).waitFor();
    assert.equal(await page.evaluate(() => navigator.clipboard.readText()), "Historical notice 0");
    await page.keyboard.press("Escape");
    assert.equal(await page.getByRole("dialog").count(), 0);
    notifications = [{ id: "new", createdAt: new Date().toISOString(), level: "system", message: "New system notice" }, ...notifications];
    await page.locator(".notification-toast").waitFor();
    await page.waitForTimeout(250);
    const bounds = await page.locator(".notification-toast").boundingBox();
    assert.ok(bounds.y >= 64);
    assert.ok(bounds.x + bounds.width <= 1280 - 240);
    assert.equal(await page.locator(".notification-progress").evaluate((node) => getComputedStyle(node).animationDuration), "10s");
    await page.locator(".notification-toast").getByRole("button", { name: "Copy notification text", exact: true }).click();
    await page.locator(".notification-toast").getByRole("button", { name: "Notification copied", exact: true }).waitFor();
    assert.equal(await page.evaluate(() => navigator.clipboard.readText()), "New system notice");
    await page.waitForTimeout(500);
    assert.equal(await page.locator(".notification-toast").count(), 1);
    await page.locator(".notification-toast").waitFor({ state: "detached", timeout: 11000 });
    await page.getByRole("button", { name: "Notifications", exact: true }).click();
    assert.equal(await page.locator(".notification-entry").count(), 13);
    assert.deepEqual(errors, []);
  } finally { await browser.close(); await server.close(); }
}, { timeout: 30000 });
