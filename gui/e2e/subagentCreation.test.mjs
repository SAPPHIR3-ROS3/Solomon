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

async function openSubagents(characteristics = ["reasoning"]) {
  const page = await browser.newPage({ viewport: { width: 1200, height: 900 } });
  page.setDefaultTimeout(10000);
  let subagents = [];
  let submitted;
  await page.route("**/__solomon/**", async route => {
    const path = new URL(route.request().url()).pathname;
    let json = {};
    if (path === "/__solomon/models") json = { providers: [{ provider: "p", models: ["m", "other"], disabled: [], metadata: {} }], current: {}, recent: [] };
    if (path === "/__solomon/roles-table") json = { rolesTable: { catalog: [{ id: "reasoning", label: "Reasoning" }], characteristics, max: 5 } };
    if (path === "/__solomon/subagents") {
      if (route.request().method() === "POST") {
        submitted = route.request().postDataJSON();
        subagents = [{ id: "p:m:0", title: submitted.model, badge: submitted.provider, detail: submitted.detail, scores: submitted.scores.map(score => ({ ...score, label: "Reasoning" })) }];
      }
      json = { subagents };
    }
    if (path === "/__solomon/projects") json = { projects: [] };
    await route.fulfill({ json });
  });
  await page.goto(url);
  await page.getByRole("button", { name: "Expand side panel", exact: true }).click();
  await page.getByRole("button", { name: "Customization", exact: true }).click();
  const backdrop = page.getByRole("button", { name: "Close side panels", exact: true });
  if (await backdrop.isVisible()) await backdrop.click();
  await page.getByRole("tab", { name: "Subagents", exact: true }).click();
  await page.getByRole("button", { name: "+ New", exact: true }).click();
  return { page, submitted: () => submitted };
}

test("GUI creates a subagent with model, description and table scores", async () => {
  const { page, submitted } = await openSubagents();
  const form = page.getByRole("form", { name: "New subagent" });
  await form.getByRole("combobox", { name: "Provider", exact: true }).click();
  await form.getByRole("option", { name: "p", exact: true }).click();
  await form.getByRole("combobox", { name: "Model", exact: true }).click();
  await form.getByRole("option", { name: "m", exact: true }).click();
  await form.getByLabel("Description (optional)").fill("Explorer");
  await form.getByLabel("Reasoning").fill("80");
  await form.getByRole("button", { name: "Create", exact: true }).click();
  await form.waitFor({ state: "detached" });
  assert.deepEqual(submitted(), { provider: "p", model: "m", detail: "Explorer", scores: [{ id: "reasoning", value: 80 }] });
  await page.getByText("Explorer", { exact: true }).waitFor();
});

test("GUI explains missing table and allows cancellation", async () => {
  const { page, submitted } = await openSubagents([]);
  const form = page.getByRole("form", { name: "New subagent" });
  await form.getByRole("alert").waitFor();
  assert.match(await form.getByRole("alert").textContent(), /reference table/);
  assert.equal(await form.getByRole("button", { name: "Create", exact: true }).isDisabled(), true);
  await form.getByRole("button", { name: "Cancel", exact: true }).click();
  await form.waitFor({ state: "detached" });
  assert.equal(submitted(), undefined);
  assert.equal(await page.getByRole("button", { name: "Table", exact: true }).isVisible(), true);
});

test("Subagent controls use shared styling and score arrows respect limits", async () => {
  const { page } = await openSubagents();
  const form = page.getByRole("form", { name: "New subagent" });
  const provider = form.getByRole("combobox", { name: /Provider/ });
  const style = await provider.evaluate((element) => {
    const css = getComputedStyle(element);
    return { appearance: css.appearance, image: css.backgroundImage, height: css.height };
  });
  assert.equal(style.appearance, "none");
  assert.equal(await provider.locator("svg").count(), 1);
  assert.equal(style.height, "44px");
  await provider.press("ArrowDown");
  await form.getByRole("listbox", { name: "Provider", exact: true }).waitFor();
  await provider.press("Enter");
  assert.equal(await provider.innerText(), "p");
  const model = form.getByRole("combobox", { name: "Model", exact: true });
  await model.press("ArrowDown");
  await model.press("End");
  await model.press("Enter");
  assert.equal(await model.innerText(), "other");
  await model.click();
  await model.press("Escape");
  assert.equal(await model.getAttribute("aria-expanded"), "false");
  await model.click();
  await form.getByRole("heading", { name: "New subagent" }).click();
  assert.equal(await model.getAttribute("aria-expanded"), "false");
  assert.equal(await form.locator("select").count(), 0);
  const score = form.getByLabel("Reasoning");
  await form.getByRole("button", { name: "Increase score" }).click();
  assert.equal(await score.inputValue(), "1");
  await score.fill("100");
  await form.getByRole("button", { name: "Increase score" }).click();
  assert.equal(await score.inputValue(), "100");
  await score.fill("0");
  await form.getByRole("button", { name: "Decrease score" }).click();
  assert.equal(await score.inputValue(), "0");
  await score.press("ArrowUp");
  assert.equal(await score.inputValue(), "1");
  await page.close();
});
