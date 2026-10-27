import assert from "node:assert/strict";
import { after, test } from "node:test";
import { existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { preview } from "vite";
import { chromium } from "playwright-core";

const server = await preview({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, preview: { host: "127.0.0.1", port: 0 } });

const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/chromium-browser", "/usr/bin/google-chrome"].find(existsSync);
const browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
after(async () => { await browser.close(); await new Promise(resolve => server.httpServer.close(resolve)); });
const url = `http://127.0.0.1:${server.httpServer.address().port}`;

test("New creates a rule, preserves failures and supports cancellation", async () => {
  const page = await browser.newPage();
  let rules = [];
  let fail = true;
  await page.route("**/__solomon/**", async route => {
    const endpoint = new URL(route.request().url()).pathname;
    if (endpoint === "/__solomon/rules") {
      if (route.request().method() === "POST") {
        if (fail) { await route.fulfill({ status: 500, json: {} }); return; }
        rules = [{ id: 1, text: route.request().postDataJSON().text.trim() }];
      }
      await route.fulfill({ json: { rules } }); return;
    }
    await route.fulfill({ json: { projects: [], providers: [], subagents: [], skills: [], mcps: [] } });
  });
  await page.goto(url);
  await page.getByRole("button", { name: "Expand side panel", exact: true }).click();
  await page.getByRole("button", { name: "Agents Customization", exact: true }).click();
  const backdrop = page.getByRole("button", { name: "Close side panels", exact: true });
  if (await backdrop.isVisible()) await backdrop.click();
  await page.getByRole("tab", { name: "Rules", exact: true }).click();
  await page.getByRole("button", { name: "+ New", exact: true }).click();
  const form = page.getByRole("form", { name: "New rule" });
  assert.equal(await form.getByRole("button", { name: "Create", exact: true }).isDisabled(), true);
  await form.getByLabel("Rule text").fill("Use concise explanations");
  await form.getByRole("button", { name: "Create", exact: true }).click();
  await form.getByRole("alert").waitFor();
  assert.equal(await form.getByLabel("Rule text").inputValue(), "Use concise explanations");
  fail = false;
  await form.getByRole("button", { name: "Create", exact: true }).click();
  await form.waitFor({ state: "detached" });
  await page.getByText("Use concise explanations", { exact: true }).waitFor();
  assert.equal(rules.length, 1);
  await page.getByRole("button", { name: "+ New", exact: true }).click();
  await form.getByRole("button", { name: "Cancel", exact: true }).click();
  await form.waitFor({ state: "detached" });
  assert.equal(rules.length, 1);
  await page.close();
});
