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

for (const valid of [true, false]) {
  test(`skill Edit ${valid ? "loads Markdown and saves changes" : "keeps the page usable when Markdown is missing"}`, async () => {
    const page = await openPage("/agents/customization");
    const errors = [];
    page.on("pageerror", error => errors.push(error.message));
    const skill = { id: "fixture", title: "Fixture skill", detail: "Fixture description" };
    let saved;
    await page.route("**/__solomon/skills*", route => {
      const request = route.request();
      if (request.method() === "PATCH") {
        saved = request.postDataJSON().content;
        return route.fulfill({ json: { skills: [skill] } });
      }
      const editing = new URL(request.url()).searchParams.has("id");
      return route.fulfill({ json: editing && valid ? { content: "---\nname: Fixture skill\n---\nOriginal instructions" } : { skills: [skill] } });
    });
    try {
      await page.getByRole("tab", { name: "Skills", exact: true }).click();
      await page.getByRole("button", { name: "More options for skill Fixture skill", exact: true }).click();
      await page.getByRole("menuitem", { name: "Edit", exact: true }).click();
      if (valid) {
        const editor = page.getByRole("textbox", { name: "Skill Markdown", exact: true });
        await editor.waitFor();
        assert.match(await editor.inputValue(), /Original instructions/);
        await editor.fill("Updated instructions");
        await page.getByRole("button", { name: "Save", exact: true }).click();
        await page.getByRole("button", { name: "More options for skill Fixture skill", exact: true }).waitFor();
        assert.equal(saved, "Updated instructions");
      } else {
        await page.getByRole("alert").filter({ hasText: "did not return the skill Markdown" }).waitFor();
        await page.getByRole("button", { name: "More options for skill Fixture skill", exact: true }).waitFor();
      }
      assert.deepEqual(errors, []);
    } finally { await page.close(); }
  });
}
