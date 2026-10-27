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

async function openPage(path = "/", width = 1100) {
  const page = await browser.newPage({ viewport: { width, height: 900 } });
  page.setDefaultTimeout(10000);
  await page.route("**/__solomon/**", route => route.fulfill({ json: { projects: [], providers: [], subagents: [], skills: [], mcps: [], rules: [], groups: [], agents: [] } }));
  await page.goto(url + path);
  return page;
}

async function navigate(page, label) {
  const expand = page.getByRole("button", { name: "Expand side panel", exact: true });
  if (await expand.isVisible()) await expand.click();
  await page.getByRole("button", { name: label, exact: true }).click();
}

for (const [path, name] of [["/agents", "Active agents"], ["/agents/customization", "Agents Customization"]]) {
  test(`${path} opens directly and survives reload`, async () => {
    const page = await openPage(path);
    try {
      await page.getByRole("region", { name, exact: true }).waitFor();
      await page.reload();
      await page.getByRole("region", { name, exact: true }).waitFor();
      assert.equal(new URL(page.url()).pathname, path);
    } finally { await page.close(); }
  });
}

test("agent pages follow browser history and close desktop overlay panels", async () => {
  const page = await openPage();
  try {
    await navigate(page, "Active agents");
    await page.getByRole("region", { name: "Active agents", exact: true }).waitFor();
    assert.equal(new URL(page.url()).pathname, "/agents");
    assert.equal(await page.locator(".side-panel").count(), 0);
    await navigate(page, "Agents Customization");
    await page.getByRole("region", { name: "Agents Customization", exact: true }).waitFor();
    assert.equal(new URL(page.url()).pathname, "/agents/customization");
    assert.equal(await page.locator(".side-panel").count(), 0);
    await page.goBack();
    await page.getByRole("region", { name: "Active agents", exact: true }).waitFor();
    await page.goForward();
    await page.getByRole("region", { name: "Agents Customization", exact: true }).waitFor();
    await navigate(page, "User settings");
    await page.getByRole("region", { name: "Settings", exact: true }).waitFor();
    assert.equal(new URL(page.url()).pathname, "/settings");
    await page.goBack();
    await page.getByRole("region", { name: "Agents Customization", exact: true }).waitFor();
    await navigate(page, "Agents Customization");
    await page.waitForURL(url + "/");
    assert.equal(await page.locator(".customization-screen").count(), 0);
  } finally { await page.close(); }
});

test("wide desktop navigation preserves docked panels", async () => {
  const page = await openPage("/", 1920);
  try {
    await navigate(page, "Agents Customization");
    await page.getByRole("region", { name: "Agents Customization", exact: true }).waitFor();
    assert.equal(await page.locator(".side-panel").count(), 1);
    assert.equal(await page.getByRole("button", { name: "Agents Customization", exact: true }).getAttribute("aria-pressed"), "true");
  } finally { await page.close(); }
});
