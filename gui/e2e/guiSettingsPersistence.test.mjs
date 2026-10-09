import assert from "node:assert/strict";
import { after, test } from "node:test";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { existsSync } from "node:fs";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createServer } from "node:net";
import { chromium } from "playwright-core";

// Run against a newly built CLI with packaged frontend assets:
// SOLOMON_TEST_BINARY=/path/to/new/solomon node --test e2e/guiSettingsPersistence.test.mjs
const cli = process.env.SOLOMON_TEST_BINARY || "solomon";
const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/chromium-browser", "/usr/bin/google-chrome"].find(existsSync);
assert.ok(executablePath, "Install Chromium or set SOLOMON_TEST_BROWSER");
const home = await mkdtemp(join(tmpdir(), "solomon-gui-config-production-"));
const configPath = join(home, "config.toml");
await writeFile(configPath, "user_name = 'Fixture'\n[gui.chat]\nauto_close_tool_calls = false\nstart_tool_calls_collapsed = false\n");
const listener = createServer();
listener.listen(0, "127.0.0.1");
await once(listener, "listening");
const port = listener.address().port;
await new Promise(resolve => listener.close(resolve));
const url = `http://127.0.0.1:${port}`;
const daemon = spawn(cli, ["server", "run"], {
  cwd: home,
  env: { ...process.env, SOLOMON_HOME: home, SOLOMON_SERVER_PORT: String(port) },
  stdio: ["ignore", "pipe", "pipe"],
});
let log = "";
let startupError;
daemon.stdout.on("data", data => { log += data; });
daemon.stderr.on("data", data => { log += data; });
daemon.on("error", error => { startupError = error; });
after(async () => {
  if (daemon.pid && daemon.exitCode === null) {
    const exited = once(daemon, "exit");
    daemon.kill("SIGTERM");
    await Promise.race([exited, new Promise(resolve => setTimeout(resolve, 5000))]);
    if (daemon.exitCode === null) { daemon.kill("SIGKILL"); await exited; }
  }
  await rm(home, { recursive: true, force: true });
});
for (let attempt = 0; attempt < 200; attempt++) {
  if (startupError) throw startupError;
  if (daemon.exitCode !== null) throw new Error(log);
  try { if ((await (await fetch(`${url}/health`)).json()).ok) break; } catch {}
  await new Promise(resolve => setTimeout(resolve, 50));
}
const response = await fetch(`${url}/__solomon/gui-settings`);
assert.ok(response.headers.get("content-type")?.includes("application/json"), "Rebuild the daemon and GUI assets before running this test");
const browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
after(() => browser.close());

test("packaged Settings page saves to daemon config and reloads it in a fresh browser", async () => {
  const openSettings = async () => {
    const page = await browser.newPage({ viewport: { width: 1200, height: 900 } });
    page.setDefaultTimeout(10000);
    await page.route("**/__solomon/models*", route => route.fulfill({ json: { current: { provider: "", model: "" }, providers: [], recent: [] } }));
    await page.goto(url);
    await page.getByRole("button", { name: "Expand side panel", exact: true }).click();
    await page.getByRole("button", { name: "User settings", exact: true }).click();
    await page.getByRole("button", { name: "Chat", exact: true }).click();
    return page;
  };
  const page = await openSettings();
  after(() => page.close());
  const auto = page.getByRole("switch", { name: "Auto-close tool calls" });
  await auto.click();
  await page.waitForFunction(() => document.querySelector('[aria-labelledby="auto-close-tool-calls-label"]').getAttribute("aria-checked") === "true");
  assert.match(await readFile(configPath, "utf8"), /\[gui.chat\][\s\S]*auto_close_tool_calls = true/);
  const saved = await (await fetch(`${url}/__solomon/gui-settings`)).json();
  assert.equal(saved.chat.autoCloseToolCalls, true);
  assert.equal(await page.evaluate(() => localStorage.getItem("solomon.chat.auto-collapse-tool-calls")), null);
  const freshPage = await openSettings();
  after(() => freshPage.close());
  await freshPage.waitForFunction(() => document.querySelector('[aria-labelledby="auto-close-tool-calls-label"]').getAttribute("aria-checked") === "true");
  const updated = (await readFile(configPath, "utf8")).replace("start_tool_calls_collapsed = false", "start_tool_calls_collapsed = true");
  await writeFile(configPath, updated);
  await freshPage.evaluate(() => window.dispatchEvent(new Event("focus")));
  await freshPage.waitForFunction(() => document.querySelector('[aria-labelledby="start-tool-calls-collapsed-label"]').getAttribute("aria-checked") === "true");
  assert.equal(await readFile(configPath, "utf8"), updated);
});


test("desktop bridge saves chat preferences through a browser CORS preflight", async () => {
  const page = await browser.newPage({ viewport: { width: 1200, height: 900 } });
  after(() => page.close());
  page.setDefaultTimeout(15000);
  await writeFile(configPath, "user_name = 'Fixture'\n[gui.chat]\nauto_close_tool_calls = false\nstart_tool_calls_collapsed = false\n");
  await page.addInitScript(({ url }) => {
    window.go = { main: { ServerBridge: { URL: async () => url } } };
  }, { url });
  await page.route("**/__solomon/models*", route => route.fulfill({ json: { current: { provider: "", model: "" }, providers: [], recent: [] } }));
  await page.goto(url.replace("127.0.0.1", "localhost"));
  await page.getByRole("button", { name: "Expand side panel", exact: true }).click();
  await page.getByRole("button", { name: "User settings", exact: true }).click();
  await page.getByRole("button", { name: "Chat", exact: true }).click();
  await page.getByRole("switch", { name: "Auto-close tool calls" }).click();
  await page.waitForFunction(() => {
    const toggle = document.querySelector('[aria-labelledby="auto-close-tool-calls-label"]');
    return toggle && !toggle.disabled && toggle.getAttribute("aria-checked") === "true";
  });
  assert.match(await readFile(configPath, "utf8"), /auto_close_tool_calls = true/);
  assert.equal(await page.getByRole("alert").count(), 0);
});
