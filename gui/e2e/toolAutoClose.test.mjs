import assert from "node:assert/strict";
import { after, test } from "node:test";
import { existsSync } from "node:fs";
import { mkdtemp, readFile, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { execFile, spawn } from "node:child_process";
import { promisify } from "node:util";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { chromium } from "playwright-core";

const home = await mkdtemp(join(tmpdir(), "solomon-gui-settings-test-"));
after(() => rm(home, { recursive: true, force: true }));
const configPath = join(home, "config.toml");
const helper = join(home, process.platform === "win32" ? "gui-settings-helper.exe" : "gui-settings-helper");
await promisify(execFile)("go", ["build", "-o", helper, "gui/desktop/gui_settings.go"], { cwd: fileURLToPath(new URL("../..", import.meta.url)) });
const baseConfig = "user_name = 'Fixture'\n[providers.OpenAI]\napi_key = 'secret-fixture'\n";
await writeFile(configPath, baseConfig);
let failNextWrite = false;

const fixtureSource = `
        import { createElement } from 'react';
        import { createRoot } from 'react-dom/client';
        import { SettingsPage } from '/src/settings/SettingsPage.tsx';
        import { ChatMessageGroups } from '/src/chat/ChatMessageGroups.tsx';
        import { indexChatMessages } from '/src/chat/chatMessageUtils.ts';
        import { liveChatFromPayload, applyLiveStreamEvent } from '/src/chat/chatClient.ts';
        import { setAutoCollapseToolCalls, setStartToolCallsCollapsed } from '/src/settings/chatPreferences.ts';
        const settings = createRoot(document.getElementById('settings'));
        const chat = createRoot(document.getElementById('chat'));
        settings.render(createElement(SettingsPage, { onHome() {} }));
        window.renderChat = (isWorking, name = 'shell', final = true, interrupted = false, turn = 'tool-message') => {
          const messages = [{ id: turn, role: 'assistant', content: '',
            toolCalls: [{ id: 'tool', name, intent: 'Test tool',
              status: 'success', result: { status: 'success', output: 'done' } }] }];
          if (final) messages.push({ id: 'final', role: 'assistant', content: 'Final response',
            status: interrupted ? 'interrupted' : undefined });
          chat.render(createElement(ChatMessageGroups, { isWorking, messages: indexChatMessages(liveChatFromPayload({ messages }, 'test').messages) }));
        };
        window.setPreferences = async (autoClose, startClosed) => {
          await setAutoCollapseToolCalls(autoClose);
          await setStartToolCallsCollapsed(startClosed);
        };
        window.startLiveChat = async () => {
          const { ChatView } = await import('/src/chat/ChatView.tsx');
          document.getElementById('settings').style.display = 'none';
          let liveChat = liveChatFromPayload({ id: 'live', mode: 'agent', status: 'running',
            messages: [{ id: 'live-tools', role: 'assistant', content: '', toolCalls: [] }] }, 'test');
          const render = () => chat.render(createElement(ChatView, {
            chat: liveChat, isStreaming: true, onDeleteMessage() {}, onSend() {}, onStopTool() {}, onStopStreaming() {},
          }));
          window.emitChatEvent = event => { liveChat = applyLiveStreamEvent(liveChat, event); render(); };
          render();
        };
        window.renderChat(true);
`;

const server = await createServer({
  plugins: [{
    name: "tool-auto-close-fixture",
    configureServer(server) {
      server.middlewares.use("/__solomon/gui-settings", (request, response) => {
        response.setHeader("Content-Type", "application/json");
        if (request.method === "PATCH" && failNextWrite) {
          failNextWrite = false; response.statusCode = 500;
          response.end(JSON.stringify({ error: "Fixture write failed" })); return;
        }
        let body = "";
        request.on("data", chunk => { body += chunk; });
        request.on("end", () => {
          const payload = request.method === "GET" ? { action: "read" } : { ...JSON.parse(body), action: "patch" };
          const child = spawn(helper, [], { env: { ...process.env, SOLOMON_HOME: home }, stdio: ["pipe", "pipe", "pipe"] });
          let output = "", error = "";
          child.stdout.on("data", chunk => { output += chunk; });
          child.stderr.on("data", chunk => { error += chunk; });
          child.on("close", code => {
            response.statusCode = code === 0 ? 200 : 500;
            response.end(code === 0 ? output : JSON.stringify({ error }));
          });
          child.stdin.end(JSON.stringify(payload));
        });
      });
    },
    resolveId(id) { if (id === "/__tool_fixture.js") return id; },
    load(id) { if (id === "/__tool_fixture.js") return fixtureSource; },
  }],
  root: fileURLToPath(new URL("..", import.meta.url)),
  configFile: false,
  server: { host: "127.0.0.1", port: 0, hmr: false },
  esbuild: { jsx: "automatic" },
});
await server.listen();
after(() => server.close());
const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/chromium-browser", "/usr/bin/google-chrome"].find(existsSync);
assert.ok(executablePath, "Install Chromium or set SOLOMON_TEST_BROWSER");
const browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
after(() => browser.close());

test("auto-close waits for completion, allows reopening, and persists the setting", async () => {
  const page = await browser.newPage();
  after(() => page.close());
  page.setDefaultTimeout(10000);
  page.on("pageerror", error => assert.fail(error.message));
  await page.route("**/__tool_fixture", route => route.fulfill({
    contentType: "text/html",
    body: `<div id="settings"></div><div id="chat"></div>
      <script type="module" src="/__tool_fixture.js"></script>`,
  }));
  await page.goto(`${server.resolvedUrls.local[0]}__tool_fixture`);
  const search = page.getByRole("searchbox", { name: "Search settings" });
  const toggle = page.getByRole("switch", { name: "Auto-close tool calls" });
  // Search finds the setting from the landing page by either title or description.
  await search.fill(" AUTO-CLOSE ");
  await toggle.waitFor();
  await search.fill("REOPEN IT AT ANY TIME");
  await toggle.waitFor();
  await search.fill("no matching setting");
  await page.getByText("No chat settings match this search.").waitFor();
  assert.equal(await toggle.count(), 0);
  await search.fill("");
  await page.locator('[aria-label="Chat settings"]').waitFor({ state: "detached" });
  await page.getByRole("button", { name: "Chat", exact: true }).click();
  assert.equal(await toggle.getAttribute("aria-checked"), "false");
  const collapse = page.getByRole("button", { name: "Collapse tool calls", exact: true });
  await collapse.waitFor();

  const startClosed = page.getByRole("switch", { name: "Start tool calls collapsed" });
  assert.equal(await startClosed.getAttribute("aria-checked"), "false");
  assert.equal(await page.locator(".chat-tool-card details").getAttribute("open"), "");
  const show = page.getByRole("button", { name: "Show 1 tool calls", exact: true });

  // Neither option: new ordinary tools start open and remain open on completion.
  await page.evaluate(() => window.renderChat(false));
  await collapse.waitFor();
  await changeSwitch(page, toggle);
  await page.getByRole("button", { name: "Show 1 tool calls", exact: true }).waitFor();

  // Only auto-close: successful tools and final text stay open until the run ends.
  await page.evaluate(() => window.renderChat(true, 'shell', true, false, 'turn-2'));
  await collapse.waitFor();
  assert.equal(await page.locator(".chat-tool-card details").getAttribute("open"), "");
  await page.evaluate(() => window.renderChat(false, 'shell', true, false, 'turn-2'));
  await show.waitFor();
  assert.equal(await page.locator(".chat-tool-card").count(), 0);
  assert.equal(await page.getByText("Final response", { exact: true }).count(), 1);
  await show.click();
  await collapse.waitFor();
  await page.evaluate(() => window.renderChat(false, 'shell', true, false, 'turn-2'));
  await collapse.waitFor(); // Manual reopening survives subsequent updates.

  // Only start-closed: reopening during work is preserved when the run ends.
  await changeSwitch(page, toggle);
  await changeSwitch(page, startClosed);
  await page.evaluate(() => window.renderChat(true, 'orchestrate', false, false, 'turn-3'));
  await show.waitFor();
  await show.click();
  await collapse.waitFor();
  assert.equal(await page.locator(".chat-tool-card details").getAttribute("open"), null);
  await page.locator(".chat-tool-card summary").click();
  await page.evaluate(() => window.renderChat(false, 'orchestrate', true, false, 'turn-3'));
  await collapse.waitFor();
  assert.equal(await page.locator(".chat-tool-card details").getAttribute("open"), "");

  // Both options: new tools start closed and manually reopened activity closes at the end.
  await changeSwitch(page, toggle);
  await page.evaluate(() => window.renderChat(true, 'shell', false, false, 'turn-4'));
  await show.waitFor();
  await show.click();
  await collapse.waitFor();
  await page.evaluate(() => window.renderChat(false, 'shell', true, true, 'turn-4'));
  await show.waitFor();

  await page.reload();
  await page.getByRole("button", { name: "Chat", exact: true }).click();
  await waitForSwitch(page, toggle, true);
  await waitForSwitch(page, startClosed, true);
  const saved = await readFile(configPath, "utf8");
  assert.match(saved, /\[gui.chat\]/);
  assert.match(saved, /auto_close_tool_calls = true/);
  assert.match(saved, /start_tool_calls_collapsed = true/);
  assert.equal(await page.evaluate(() => localStorage.getItem("solomon.chat.auto-collapse-tool-calls")), null);
  await show.waitFor();
  await changeSwitch(page, startClosed);
  await collapse.waitFor();
  await page.evaluate(() => window.renderChat(false));
  await show.waitFor();
  await show.click();
  await collapse.waitFor();
  const configBeforeFailure = await readFile(configPath, "utf8");
  failNextWrite = true;
  await toggle.click();
  await page.getByRole("alert").filter({ hasText: "Fixture write failed" }).waitFor();
  assert.equal(await toggle.getAttribute("aria-checked"), "true");
  assert.equal(await readFile(configPath, "utf8"), configBeforeFailure);
  await page.addStyleTag({ url: `${server.resolvedUrls.local[0]}src/styles.css?direct` });
  const sizes = await page.evaluate(() => ({
    title: parseFloat(getComputedStyle(document.getElementById("auto-close-tool-calls-label")).fontSize),
    description: parseFloat(getComputedStyle(document.getElementById("auto-close-tool-calls-description")).fontSize),
  }));
  assert.equal(sizes.description, 11);
  assert.ok(sizes.description < sizes.title);
});


test("daemon run_end closes tools even while the stream connection is still active", async () => {
  const page = await browser.newPage();
  after(() => page.close());
  page.setDefaultTimeout(10000);
  page.on("pageerror", error => assert.fail(error.message));
  await page.route("**/__tool_fixture", route => route.fulfill({
    contentType: "text/html",
    body: '<div id="settings"></div><div id="chat"></div><script type="module" src="/__tool_fixture.js"></script>',
  }));
  await page.goto(`${server.resolvedUrls.local[0]}__tool_fixture`);
  await page.waitForFunction(() => typeof window.startLiveChat === "function");
  await page.evaluate(async () => {
    await window.setPreferences(true, false);
    await window.startLiveChat();
    window.emitChatEvent({ type: "tool_start", id: "live-tool", name: "shell", arguments: { intent: "Test live tool", command: "echo done" } });
  });
  const collapse = page.getByRole("button", { name: "Collapse tool calls", exact: true });
  await collapse.waitFor();
  assert.equal(await page.locator(".chat-tool-card details").getAttribute("open"), "");
  await page.evaluate(() => {
    window.emitChatEvent({ type: "tool_result", id: "live-tool", result: { status: "success", output: "done" } });
    window.emitChatEvent({ type: "assistant_start", turn: 2 });
    window.emitChatEvent({ type: "assistant_delta", channel: "content", delta: "Live final response" });
  });
  await page.getByText("Live final response", { exact: true }).waitFor();
  await collapse.waitFor();
  await page.evaluate(() => window.emitChatEvent({ type: "run_end", exit_code: 0 }));
  await page.getByRole("button", { name: "Show 1 tool calls", exact: true }).waitFor();
  assert.equal(await page.locator(".chat-tool-card").count(), 0);
});



test("browser migration is one-time and existing config values take precedence", async () => {
  await writeFile(configPath, baseConfig);
  const page = await browser.newPage();
  after(() => page.close());
  page.setDefaultTimeout(10000);
  await page.addInitScript(() => {
    localStorage.setItem("solomon.chat.auto-collapse-tool-calls", "true");
    localStorage.setItem("solomon.chat.start-tool-calls-collapsed", "true");
  });
  await page.route("**/__tool_fixture", route => route.fulfill({
    contentType: "text/html",
    body: '<div id="settings"></div><div id="chat"></div><script type="module" src="/__tool_fixture.js"></script>',
  }));
  await page.goto(`${server.resolvedUrls.local[0]}__tool_fixture`);
  await page.getByRole("button", { name: "Chat", exact: true }).click();
  const auto = page.getByRole("switch", { name: "Auto-close tool calls" });
  const start = page.getByRole("switch", { name: "Start tool calls collapsed" });
  await waitForSwitch(page, auto, true);
  await waitForSwitch(page, start, true);
  assert.match(await readFile(configPath, "utf8"), /auto_close_tool_calls = true/);
  await writeFile(configPath, baseConfig + "\n[gui.chat]\nauto_close_tool_calls = false\nstart_tool_calls_collapsed = false\n");
  await page.reload();
  await page.getByRole("button", { name: "Chat", exact: true }).click();
  await waitForSwitch(page, auto, false);
  await waitForSwitch(page, start, false);
  assert.equal(await page.evaluate(() => localStorage.getItem("solomon.chat.auto-collapse-tool-calls")), null);
  await page.getByRole("button", { name: "Collapse tool calls", exact: true }).waitFor();
  // Completed transcripts mounted after the finish event must also auto-close.
  await changeSwitch(page, auto);
  await page.evaluate(() => window.renderChat(false, 'shell', true, false, 'finished-before-mount'));
  await page.getByRole("button", { name: "Show 1 tool calls", exact: true }).waitFor();
});

async function waitForSwitch(page, toggle, enabled) {
  const label = await toggle.getAttribute("aria-labelledby");
  await page.waitForFunction(({ label, enabled }) => {
    const button = document.querySelector('[aria-labelledby="' + label + '"]');
    return button && !button.disabled && button.getAttribute("aria-checked") === String(enabled);
  }, { label, enabled });
}

async function changeSwitch(page, toggle) {
  const enabled = (await toggle.getAttribute("aria-checked")) !== "true";
  await toggle.click();
  await waitForSwitch(page, toggle, enabled);
}

test("a stalled save times out, releases both switches, and can be retried", async () => {
  const page = await browser.newPage();
  after(() => page.close());
  page.setDefaultTimeout(15000);
  await page.route("**/__tool_fixture", route => route.fulfill({
    contentType: "text/html",
    body: '<div id="settings"></div><div id="chat"></div><script type="module" src="/__tool_fixture.js"></script>',
  }));
  await page.goto(`${server.resolvedUrls.local[0]}__tool_fixture`);
  await page.getByRole("button", { name: "Chat", exact: true }).click();
  const auto = page.getByRole("switch", { name: "Auto-close tool calls" });
  await waitForSwitch(page, auto, true);
  const before = await readFile(configPath, "utf8");
  await page.route("**/__solomon/gui-settings", route => {
    if (route.request().method() !== "PATCH") return route.continue();
    // Leave the write unanswered to reproduce a connection that never settles.
  });
  await auto.click();
  await page.getByRole("alert").filter({ hasText: "Saving chat settings timed out" }).waitFor();
  await waitForSwitch(page, auto, true);
  assert.equal(await readFile(configPath, "utf8"), before);
  const styles = await page.locator(".settings-chat-error").evaluate(element => ({
    padding: getComputedStyle(element).paddingTop,
    radius: getComputedStyle(element.querySelector("button")).borderRadius,
  }));
  assert.equal(styles.padding, "12px");
  assert.equal(styles.radius, "6px");
  await page.unroute("**/__solomon/gui-settings");
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  await page.getByRole("alert").waitFor({ state: "detached" });
  await changeSwitch(page, auto);
  assert.match(await readFile(configPath, "utf8"), /auto_close_tool_calls = false/);
});
