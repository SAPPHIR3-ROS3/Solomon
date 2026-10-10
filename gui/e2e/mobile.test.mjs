import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { existsSync } from "node:fs";
import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createServer } from "node:net";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright-core";

// These checks always use a temporary Solomon home and workspace. Model
// responses are fixtures; project/file APIs and terminal PTYs use the daemon.
const guiRoot = fileURLToPath(new URL("..", import.meta.url));
const cli = process.env.SOLOMON_TEST_BINARY || "solomon";
const mode = process.env.SOLOMON_TEST_MODE || "dev";
const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/chromium-browser", "/usr/bin/google-chrome"].find(existsSync);
let browser, daemon, home, url, lanUrl, folder, project, chat;
let daemonLog = "";
const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const modelCatalog = {
  current: { provider: "Test", model: "mobile-model" }, recent: [],
  providers: [{ provider: "Test", complete: true, models: ["mobile-model"], metadata: {}, disabled: [], supportsFastMode: false, thinkingLevelModels: ["mobile-model"] }],
};
const chatMessages = [
  { id: "u-1", role: "user", content: "Hello from a phone" },
  { id: "a-1", role: "assistant", content: "Mobile response\n\n| Column A | Column B |\n|---|---|\n| A long value in a mobile table | another long value |\n\n```text\nA very long line " + "abc".repeat(80) + "\n```" },
];

async function api(path, body) {
  const response = await fetch(url + path, body === undefined ? undefined : { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  assert.ok(response.ok, `${path}: ${response.status} ${await response.clone().text()}`);
  return response.json();
}

before(async () => {
  assert.ok(executablePath, "Install Chromium or set SOLOMON_TEST_BROWSER");
  assert.ok(mode === "dev" || mode === "normal", "SOLOMON_TEST_MODE must be dev or normal");
  home = await mkdtemp(join(tmpdir(), "solomon-mobile-e2e-"));
  folder = join(home, "Mobile workspace"); await mkdir(folder); await writeFile(join(folder, "notes.txt"), "Mobile editor test\n");
  const listener = createServer(); listener.listen(0, "127.0.0.1"); await once(listener, "listening");
  const port = listener.address().port; await new Promise((resolve) => listener.close(resolve));
  url = `http://127.0.0.1:${port}`;
  daemon = spawn(cli, mode === "dev" ? ["server", "run", "dev", guiRoot] : ["server", "run"], { cwd: home, env: { ...process.env, SOLOMON_HOME: home, SOLOMON_SERVER_PORT: String(port), SOLOMON_VITE_CACHE_DIR: join(home, "vite-cache") }, stdio: ["ignore", "pipe", "pipe"] });
  let startupError; daemon.on("error", (error) => { startupError = error; });
  daemon.stdout.on("data", (data) => { daemonLog += data; }); daemon.stderr.on("data", (data) => { daemonLog += data; });
  let ready = false;
  for (let attempt = 0; attempt < 200; attempt += 1) {
    if (startupError) throw startupError;
    if (daemon.exitCode !== null) throw new Error(daemonLog);
    try { ready = (await (await fetch(url + "/health")).json()).ok; } catch {}
    if (ready) break;
    await wait(100);
  }
  assert.ok(ready, `Test daemon did not become ready: ${daemonLog}`);
  const health = await api("/health");
  lanUrl = health.server.addresses?.find((address) => address.kind === "local" && /^(en|wl)/.test(address.interface || ""))?.url;
  project = (await api("/__solomon/projects", { path: folder })).project;
  chat = await api(`/__solomon/projects/${project.id}/chats`, { title: "Mobile chat" });
  browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
}, { timeout: 30000 });

after(async () => {
  await browser?.close();
  if (daemon?.pid && daemon.exitCode === null) {
    const exited = once(daemon, "exit");
    if (process.platform === "win32") {
      const cleanup = spawn("taskkill", ["/PID", String(daemon.pid), "/T", "/F"], { windowsHide: true, stdio: "ignore" });
      await once(cleanup, "exit");
      await exited;
    } else {
      daemon.kill("SIGTERM");
      await Promise.race([exited, wait(5000)]);
      if (daemon.exitCode === null) { daemon.kill("SIGKILL"); await exited; }
    }
  }
  if (home) await rm(home, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
});

async function mobilePage(width, height = 844, pageURL = url, touch = true) {
  const page = await browser.newPage({ viewport: { width, height }, isMobile: touch, hasTouch: touch, deviceScaleFactor: 1 });
  page.setDefaultTimeout(7000);
  const errors = []; page.on("pageerror", (error) => errors.push(error.message));
  await page.route("**/__solomon/models", (route) => route.fulfill({ json: modelCatalog }));
  let snapshot = { ...chat, mode: "agent", workspaceName: project.name, messages: chatMessages };
  await page.route(`**/__solomon/projects/${project.id}/chats/${chat.id}`, (route) => route.fulfill({ json: snapshot }));
  await page.goto(pageURL);
  try { await page.locator(".welcome-stage .welcome-composer").waitFor({ state: "visible" }); } catch (error) { await page.close(); throw new Error(`${error.message}\nBrowser errors: ${JSON.stringify(errors)}\nDaemon: ${daemonLog}`); }
  return { page, errors, setSnapshot: (next) => { snapshot = next; } };
}

async function withinViewport(page, selector) {
  const rectangle = await page.locator(selector).boundingBox();
  const { width, height } = page.viewportSize();
  assert.ok(rectangle && rectangle.x >= -1 && rectangle.x + rectangle.width <= width + 1 && rectangle.y >= -1 && rectangle.y + rectangle.height <= height + 1, `${selector} exceeds ${width}x${height}: ${JSON.stringify(rectangle)}`);
}

async function openChat(page) {
  await page.getByRole("button", { name: "Expand side panel", exact: true }).tap();
  const chatButton = page.getByRole("button", { name: "Mobile chat", exact: false });
  if (!await chatButton.isVisible()) await page.getByRole("button", { name: project.name, exact: true }).tap();
  await chatButton.tap(); await page.locator(".chat-view").waitFor({ state: "visible" });
}

test("startup stays on home after opening a thread in the previous session", async () => {
  const { page, errors } = await mobilePage(390);
  let chatLoads = 0;
  page.on("request", (request) => {
    if (new URL(request.url()).pathname === `/__solomon/projects/${project.id}/chats/${chat.id}`) chatLoads += 1;
  });
  try {
    await openChat(page);
    assert.deepEqual(await page.evaluate(() => JSON.parse(localStorage.getItem("solomon.active-chat"))), { chatID: chat.id, projectID: project.id });
    const previousChatLoads = chatLoads;
    await page.reload({ waitUntil: "networkidle" });
    assert.ok(await page.locator(".welcome-stage .welcome-composer").isVisible());
    assert.equal(await page.locator(".chat-view").count(), 0);
    assert.equal(chatLoads, previousChatLoads);
    await openChat(page);
    assert.ok(await page.locator(".chat-view").isVisible());
    assert.deepEqual(errors, []);
  } finally { await page.close(); }
});

for (const [width, height] of [[320, 640], [390, 844], [412, 915], [844, 390]]) {
  test(`touch navigation and menus at ${width}x${height}`, async () => {
    const { page, errors } = await mobilePage(width, height);
    try {
      await withinViewport(page, ".welcome-stage");
      await page.getByRole("button", { name: "Expand side panel", exact: true }).tap();
      await withinViewport(page, ".side-panel");
      await page.getByRole("button", { name: "User settings", exact: true }).tap();
      await page.getByRole("button", { name: "Models", exact: true }).tap();
      await withinViewport(page, ".settings-main");
      await page.getByRole("button", { name: "Docs", exact: true }).tap();
      await page.getByRole("button", { name: "Back to home", exact: true }).tap();
      // Wider landscape layouts may retain their desktop-style drawer.
      if (await page.locator(".side-panel-backdrop").isVisible()) await page.getByRole("button", { name: "Close side panels" }).tap();
      await page.getByRole("button", { name: "Select model", exact: true }).tap();
      await withinViewport(page, ".welcome-model-menu");
      assert.ok(await page.getByRole("option", { name: /mobile-model/ }).isVisible());
      await page.getByRole("button", { name: "Select model", exact: true }).tap();
      await page.getByRole("button", { name: "Editor", exact: true }).tap();
      await withinViewport(page, ".editor-page");
      await page.getByRole("button", { name: "Agent", exact: true }).tap();
      await page.getByRole("button", { name: "Expand side panel", exact: true }).tap();
      await page.getByRole("button", { name: "Customization", exact: true }).tap();
      if (await page.locator(".side-panel-backdrop").isVisible()) await page.getByRole("button", { name: "Close side panels" }).tap();
      await page.getByRole("tab", { name: "Subagents", exact: true }).tap();
      await withinViewport(page, ".customization-screen");
      assert.deepEqual(errors, []);
    } finally { await page.close(); }
  });
}

test("mobile chat, IME, photo attachments and streamed replies", async () => {
  const { page, errors, setSnapshot } = await mobilePage(390);
  try {
    await openChat(page);
    assert.equal(await page.locator(".side-panel-backdrop").count(), 0);
    await withinViewport(page, ".chat-topbar"); await withinViewport(page, ".chat-composer-dock");
    assert.ok(await page.getByText("Mobile response", { exact: false }).isVisible());
    const input = page.locator(".chat-view textarea.at-mention-input");
    await input.fill("日本語");
    let sends = 0;
    await page.route(`**/__solomon/projects/${project.id}/chats/${chat.id}/messages`, async (route) => {
      sends += 1;
      const { content, images } = route.request().postDataJSON();
      assert.ok(images.length === 1 && images[0].data.startsWith("data:image/png;base64,"));
      const finalChat = { ...chat, mode: "agent", messages: [...chatMessages, { id: "u-2", role: "user", content }, { id: "a-2", role: "assistant", content: "Mobile stream delivered" }] };
      setSnapshot(finalChat);
      const events = [{ type: "chat_snapshot", chat: finalChat }, { type: "run_end", status: "success" }];
      await route.fulfill({ contentType: "text/event-stream", body: events.map((event) => `data: ${JSON.stringify(event)}\n\n`).join("") });
    });
    await input.evaluate((element) => element.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true, cancelable: true, isComposing: true })));
    await wait(100); assert.equal(sends, 0); assert.equal(await input.inputValue(), "日本語");
    const chooserPromise = page.waitForEvent("filechooser");
    await page.getByRole("button", { name: "Attach images", exact: true }).tap();
    await (await chooserPromise).setFiles({ name: "phone.png", mimeType: "image/png", buffer: Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/lS8AAAAASUVORK5CYII=", "base64") });
    assert.match(await input.inputValue(), /\[img-\d+\]/);
    await page.getByRole("button", { name: "Send", exact: true }).tap();
    await page.getByText("Mobile stream delivered", { exact: true }).waitFor();
    assert.equal(sends, 1); assert.deepEqual(errors, []);
  } finally { await page.close(); }
});

test("visible viewport keeps the composer above the virtual keyboard", async () => {
  const { page } = await mobilePage(390);
  try {
    const input = page.locator(".welcome-stage textarea.at-mention-input");
    await input.fill("Draft survives keyboard resize");
    await page.evaluate(() => {
      Object.defineProperty(window.visualViewport, "height", { configurable: true, value: 310 });
      window.visualViewport.dispatchEvent(new Event("resize"));
    });
    await page.waitForFunction(() => document.querySelector(".app-shell").getBoundingClientRect().height === 310);
    const composer = await page.locator(".welcome-stage .welcome-composer").boundingBox();
    assert.ok(composer.y >= 0 && composer.y + composer.height <= 311, JSON.stringify(composer));
    assert.equal(await input.inputValue(), "Draft survives keyboard resize");
    await page.evaluate(() => { delete window.visualViewport.height; window.visualViewport.dispatchEvent(new Event("resize")); });
    await page.waitForFunction(() => document.querySelector(".app-shell").getBoundingClientRect().height === innerHeight);
  } finally { await page.close(); }
});

test("phone editor opens a file and saves through the daemon", async () => {
  const { page, errors } = await mobilePage(390);
  try {
    await openChat(page); await page.getByRole("button", { name: "Editor", exact: true }).tap();
    await page.getByRole("button", { name: "notes.txt", exact: false }).tap();
    await page.locator(".cm-content").waitFor();
    assert.match(await page.locator(".editor-page").getAttribute("class"), /side-collapsed/);
    await page.locator(".cm-content").fill("Saved from mobile\n");
    await page.getByRole("button", { name: "Save file", exact: true }).tap();
    for (let attempt = 0; attempt < 30 && await readFile(join(folder, "notes.txt"), "utf8") !== "Saved from mobile\n"; attempt += 1) await wait(100);
    assert.equal(await readFile(join(folder, "notes.txt"), "utf8"), "Saved from mobile\n"); assert.deepEqual(errors, []);
  } finally { await page.close(); }
});

test("mobile terminal sends input and receives PTY output over WebSocket", async () => {
  const { page, errors } = await mobilePage(390);
  try {
    let output = "";
    page.on("websocket", (socket) => socket.on("framereceived", ({ payload }) => {
      try { const event = JSON.parse(String(payload)); if (event.type === "solomon-output") output += Buffer.from(event.data, "base64").toString(); } catch {}
    }));
    await page.getByRole("button", { name: "Show terminal panel", exact: true }).tap();
    await page.locator(".xterm-helper-textarea").waitFor({ state: "attached" });
    for (let attempt = 0; attempt < 100 && !output; attempt += 1) await wait(100);
    assert.ok(output, "Terminal did not connect");
    const command = process.platform === "win32" ? "Write-Output ('MOBILE_' + 'terminal_OK')" : "printf MOBILE_%s_OK terminal";
    await page.locator(".xterm-helper-textarea").focus(); await page.keyboard.type(command); await page.keyboard.press("Enter");
    for (let attempt = 0; attempt < 100 && !output.includes("MOBILE_terminal_OK"); attempt += 1) await wait(100);
    assert.ok(output.includes("MOBILE_terminal_OK"), output); await withinViewport(page, ".terminal-panel"); assert.deepEqual(errors, []);
  } finally { await page.close(); }
});

test("landscape keyboard preserves the draft and send button", async () => {
  const { page } = await mobilePage(844, 390);
  try {
    const input = page.locator(".welcome-stage textarea.at-mention-input");
    await input.fill("Landscape draft");
    await page.evaluate(() => { Object.defineProperty(window.visualViewport, "height", { configurable: true, value: 230 }); window.visualViewport.dispatchEvent(new Event("resize")); });
    await page.waitForFunction(() => document.querySelector(".app-shell").getBoundingClientRect().height === 230);
    assert.equal(await input.inputValue(), "Landscape draft");
    const composer = await page.locator(".welcome-stage .welcome-composer").boundingBox();
    assert.ok(composer.y + composer.height <= 231, JSON.stringify(composer));
  } finally { await page.close(); }
});

test("desktop navigation still works with the responsive additions", async () => {
  const { page, errors } = await mobilePage(1440, 900, url, false);
  try {
    await withinViewport(page, ".welcome-stage");
    await page.getByRole("button", { name: "Expand side panel", exact: true }).click();
    await page.getByRole("button", { name: "User settings", exact: true }).click();
    await page.getByRole("button", { name: "Back to home", exact: true }).click();
    if (await page.locator(".side-panel-backdrop").isVisible()) await page.getByRole("button", { name: "Close side panels" }).click();
    await page.getByRole("button", { name: "Editor", exact: true }).click();
    await withinViewport(page, ".editor-page"); assert.deepEqual(errors, []);
  } finally { await page.close(); }
});

test("phone access uses the LAN host for API and terminal connections", async (context) => {
  if (!lanUrl) { context.skip("No LAN interface available"); return; }
  const { page, errors } = await mobilePage(390, 844, lanUrl);
  try {
    assert.equal(await page.evaluate(() => isSecureContext), false);
    const data = await page.evaluate(async () => (await fetch("/__solomon/projects")).json());
    assert.ok(data.projects.some((entry) => entry.id === project.id));
    await openChat(page);
    await page.getByRole("button", { name: "Copy message", exact: true }).first().tap();
    await page.getByRole("button", { name: "Message copied", exact: true }).waitFor();
    const connected = await page.evaluate((folder) => new Promise((resolve, reject) => {
      const target = new URL("/__solomon/terminal", location.href); target.protocol = "ws:"; target.searchParams.set("path", folder);
      const socket = new WebSocket(target);
      const timer = setTimeout(() => { socket.close(); reject(new Error("LAN terminal timed out")); }, 5000);
      socket.onerror = () => { clearTimeout(timer); reject(new Error("LAN terminal failed")); };
      socket.onmessage = (event) => {
        const message = JSON.parse(event.data);
        if (message.type === "solomon-terminal") { clearTimeout(timer); socket.close(); resolve(Boolean(message.id)); }
      };
    }), folder);
    assert.ok(connected); assert.deepEqual(errors, []);
  } finally { await page.close(); }
});
