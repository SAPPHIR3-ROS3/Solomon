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

test("research navigation and context menu deletion preserve usable views", async () => {
  const page = await browser.newPage({ viewport: { width: 1500, height: 950 } });
  page.setDefaultTimeout(10000);
  const project = { id: "project", name: "Workspace", path: "/workspace", chatCount: 1, chats: [{ id: "chat", title: "Original chat", lastMessageAt: "2026-11-16T00:00:00Z" }] };
  let reportRequests = 0;
  let deleteRequests = 0;
  await page.route("**/__solomon/**", route => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/__solomon/projects") return route.fulfill({ json: { projects: [project] } });
    if (path === "/__solomon/active-agents") return route.fulfill({ json: { projects: [{ id: project.id, name: project.name, agents: [{ id: "chat", kind: "chat", projectID: project.id, title: "Original chat", status: "running" }] }] } });
    if (path === "/__solomon/projects/project/chats/chat") return route.fulfill({ json: { id: "chat", title: "Original chat", messages: [{ id: "message", role: "user", content: "Keep this conversation visible" }] } });
    if (path === "/__solomon/projects/project/research/done" && route.request().method() === "DELETE") return route.fulfill({ status: 204 });
    if (path === "/__solomon/projects/project/research/failed" && route.request().method() === "DELETE") {
      deleteRequests++;
      return deleteRequests === 1 ? route.fulfill({ status: 500, json: { error: "Deletion unavailable" } }) : route.fulfill({ status: 204 });
    }
    if (path === "/__solomon/projects/project/research") return route.fulfill({ json: [{ id: "failed", title: "Failed research", status: "failed", error: "Search connection closed" }, { id: "done", title: "Completed research", status: "done" }] });
    if (path.endsWith("/report")) { reportRequests++; return route.fulfill({ contentType: "text/html", body: "<html><body>Research report</body></html>" }); }
    return route.fulfill({ json: { projects: [], providers: [], entries: [], messages: [], branches: [] } });
  });
  try {
    await page.goto(url + "/agents");
    await page.locator(".active-agents-node > button").click();
    await page.getByText("Keep this conversation visible", { exact: true }).waitFor();
    await page.getByRole("button", { name: "Expand right side panel", exact: true }).click();
    await page.getByRole("tab", { name: "Deep research", exact: true }).click();
    await page.locator(".right-side-panel-research-item").filter({ hasText: "Failed research" }).click();
    await page.getByText("Deep Research failed: Search connection closed", { exact: true }).waitFor();
    await page.getByRole("button", { name: "Dismiss notification", exact: true }).click();
    assert.equal(await page.locator(".research-report-view").count(), 0);
    assert.equal(reportRequests, 0);
    assert.equal(await page.getByText("Keep this conversation visible", { exact: true }).isVisible(), true);
    await page.locator(".right-side-panel-research-item").filter({ hasText: "Completed research" }).click();
    await page.getByRole("region", { name: "Deep research: Completed research", exact: true }).waitFor();
    const expand = page.getByRole("button", { name: "Expand side panel", exact: true });
    if (await expand.isVisible()) await expand.click();
    await page.locator(".side-panel-chat").filter({ hasText: "Original chat" }).click();
    await page.getByText("Keep this conversation visible", { exact: true }).waitFor();
    assert.equal(await page.locator(".research-report-view").count(), 0);
    const expandRight = page.getByRole("button", { name: "Expand right side panel", exact: true });
    if (await expandRight.isVisible()) await expandRight.click();
    await page.getByRole("tab", { name: "Deep research", exact: true }).click();
    const failed = page.locator(".right-side-panel-research-item").filter({ hasText: "Failed research" });
    await failed.click({ button: "right" });
    const menu = page.getByRole("menu", { name: "Deep research actions" });
    await menu.getByRole("menuitem", { name: "Delete", exact: true }).click();
    await page.waitForFunction(() => JSON.parse(localStorage.getItem("solomon:notification-errors") ?? "[]").some(entry => entry.message === "Deletion unavailable"));
    assert.equal(await failed.count(), 1);
    const dismissError = page.getByRole("button", { name: "Dismiss notification", exact: true });
    if (await dismissError.isVisible()) await dismissError.click();
    if (!(await menu.isVisible())) await failed.click({ button: "right" });
    await menu.getByRole("menuitem", { name: "Delete", exact: true }).click();
    await failed.waitFor({ state: "detached" });
    assert.equal(deleteRequests, 2);
    assert.equal(await page.locator(".research-report-view").count(), 0);
    const completed = page.locator(".right-side-panel-research-item").filter({ hasText: "Completed research" });
    await completed.focus();
    await completed.press("Shift+F10");
    await menu.waitFor();
    await page.keyboard.press("Escape");
    await menu.waitFor({ state: "detached" });
    await completed.click();
    await page.getByRole("region", { name: "Deep research: Completed research", exact: true }).waitFor();
    if (await expandRight.isVisible()) await expandRight.click();
    await page.getByRole("tab", { name: "Deep research", exact: true }).click();
    await completed.click({ button: "right" });
    await menu.getByRole("menuitem", { name: "Delete", exact: true }).click();
    await completed.waitFor({ state: "detached" });
    await page.locator(".research-report-view").waitFor({ state: "detached" });
  } finally { await page.close(); }
});

test("active research exposes Delete through its context menu", async () => {
  const page = await browser.newPage({ viewport: { width: 1500, height: 950 } });
  let deleted = false;
  await page.route("**/__solomon/**", route => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/__solomon/active-agents") return route.fulfill({ json: { projects: deleted ? [] : [{ id: "project", name: "Workspace", agents: [{ id: "research", kind: "research", title: "Running research", projectID: "project", status: "running" }] }] } });
    if (path === "/__solomon/projects/project/research/research" && route.request().method() === "DELETE") { deleted = true; return route.fulfill({ status: 204 }); }
    return route.fulfill({ json: { projects: [], providers: [] } });
  });
  try {
    await page.goto(url + "/agents");
    const research = page.locator(".active-agents-node.is-research > button");
    await research.click({ button: "right" });
    await page.getByRole("menu", { name: "Deep research actions" }).getByRole("menuitem", { name: "Delete", exact: true }).click();
    await research.waitFor({ state: "detached" });
    assert.equal(deleted, true);
    assert.equal(new URL(page.url()).pathname, "/agents");
  } finally { await page.close(); }
});
