import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { execFileSync } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdtemp, mkdir, writeFile, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import react from "@vitejs/plugin-react";
import { chromium } from "playwright-core";
const guiRoot = fileURLToPath(new URL("..", import.meta.url));
const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/chromium-browser", "/usr/bin/google-chrome"].find(existsSync);
const projectID = "a".repeat(64), previousHome = process.env.SOLOMON_HOME;
let fixture, entry, workspace, loader, server, browser, url;
const git = (...args) => execFileSync("git", ["-C", workspace, ...args], { encoding: "utf8" }).trim();
before(async () => {
  fixture = await mkdtemp(path.join(tmpdir(), "solomon-git-commit-"));
  entry = await mkdtemp(path.join(guiRoot, ".git-commit-test-"));
  workspace = path.join(fixture, "workspace");
  const home = path.join(fixture, "home");
  await mkdir(workspace); await mkdir(home); process.env.SOLOMON_HOME = home;
  await writeFile(path.join(home, "projectsId.json"), JSON.stringify({ [workspace]: projectID }));
  git("init"); git("config", "user.name", "UI Commit Test"); git("config", "user.email", "ui@example.com"); git("config", "commit.gpgsign", "false");
  await writeFile(path.join(workspace, "file.txt"), "initial"); git("add", "file.txt"); git("commit", "-m", "initial");
  loader = await createServer({ root: guiRoot, configFile: false, cacheDir: path.join(fixture, "loader"), server: { middlewareMode: true, hmr: false } });
  const { projectsPlugin } = await loader.ssrLoadModule("/projectsPlugin.ts");
  await writeFile(path.join(entry, "index.html"), '<div id="root"></div><script type="module" src="./entry.tsx"></script>');
  await writeFile(path.join(entry, "entry.tsx"), `
    import React from 'react'; import { createRoot } from 'react-dom/client';
    import { GitHistoryView } from '/src/shell/RightSidePanel.tsx';
    import { fetchProjectGitHistory, fetchProjectGitStatus, PROJECT_GIT_STATUS_CHANGED_EVENT } from '/src/projects/projects.ts';
    const project = ${JSON.stringify({ id: projectID, name: "Test", path: workspace, chats: [], chatCount: 0 })};
    function Harness() {
      const [status, setStatus] = React.useState({ staged: {}, changes: {}, isRepo: true });
      const [history, setHistory] = React.useState({ commits: [], current: '', isRepo: true });
      const [loading, setLoading] = React.useState(true);
      React.useEffect(() => {
        const refresh = () => Promise.all([fetchProjectGitStatus(project.id), fetchProjectGitHistory(project.id)]).then(([s,h]) => { setStatus(s); setHistory(h); setLoading(false); });
        void refresh(); window.addEventListener(PROJECT_GIT_STATUS_CHANGED_EVENT, refresh);
        return () => window.removeEventListener(PROJECT_GIT_STATUS_CHANGED_EVENT, refresh);
      }, []);
      return <GitHistoryView project={project} error="" gitStatus={status} gitStatusError="" gitStatusLoading={loading} history={history} loading={loading} />;
    } createRoot(document.getElementById('root')).render(<Harness />);
  `);
  server = await createServer({ root: guiRoot, configFile: false, cacheDir: path.join(fixture, "browser"), plugins: [react(), projectsPlugin()], server: { host: "127.0.0.1", port: 0, hmr: false } });
  await server.listen(); url = `http://127.0.0.1:${server.httpServer.address().port}/${path.basename(entry)}/index.html`;
  browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
});
after(async () => {
  await browser?.close(); await server?.close(); await loader?.close();
  if (previousHome === undefined) delete process.env.SOLOMON_HOME; else process.env.SOLOMON_HOME = previousHome;
  if (entry) await rm(entry, { recursive: true, force: true }); if (fixture) await rm(fixture, { recursive: true, force: true });
});
test("Commit UI creates a staged-only commit, blocks duplicate submission and refreshes history/status", async () => {
  const page = await browser.newPage();
  try {
    await page.goto(url); await page.getByLabel("Commit message", { exact: true }).fill("no staged changes");
    assert.equal(await page.getByRole("button", { name: "Commit", exact: true }).isDisabled(), true);
    await writeFile(path.join(workspace, "file.txt"), "staged"); git("add", "file.txt");
    await writeFile(path.join(workspace, "file.txt"), "unstaged"); await writeFile(path.join(workspace, "untracked.txt"), "keep");
    await page.reload(); await page.getByLabel("Commit message", { exact: true }).fill("UI staged commit");
    let release; const gate = new Promise(resolve => { release = resolve; }); let requests = 0;
    await page.route("**/commit", async route => { requests++; await gate; await route.continue(); });
    await page.getByRole("button", { name: "Commit", exact: true }).click();
    await page.getByRole("button", { name: "Committing…", exact: true }).waitFor();
    assert.equal(await page.getByRole("button", { name: "Committing…", exact: true }).isDisabled(), true);
    assert.equal(await page.getByLabel("Commit message", { exact: true }).isDisabled(), true); release();
    await page.waitForFunction(() => document.querySelector('[aria-label="Commit message"]').value === "");
    await page.getByText("UI staged commit", { exact: true }).waitFor();
    assert.equal(await page.getByRole("button", { name: "Commit", exact: true }).isDisabled(), true);
    assert.equal(requests, 1); assert.equal(git("show", "HEAD:file.txt"), "staged");
    assert.equal(await readFile(path.join(workspace, "file.txt"), "utf8"), "unstaged");
    assert.equal(await readFile(path.join(workspace, "untracked.txt"), "utf8"), "keep"); assert.equal(git("diff", "--cached", "--name-only"), "");
  } finally { await page.close(); }
});
test("Commit UI displays Git failure and retains the message and staged changes", async () => {
  git("add", "file.txt"); git("config", "user.name", ""); git("config", "user.email", ""); const head = git("rev-parse", "HEAD");
  const page = await browser.newPage();
  try {
    await page.goto(url); await page.getByLabel("Commit message", { exact: true }).fill("retain this draft");
    await page.getByRole("button", { name: "Commit", exact: true }).click(); await page.getByRole("alert").waitFor();
    assert.match(await page.getByRole("alert").textContent(), /identity|empty ident/i);
    assert.equal(await page.getByLabel("Commit message", { exact: true }).inputValue(), "retain this draft");
    await page.waitForFunction(() => !document.querySelector('[aria-label="Commit message"]').disabled);
    assert.equal(git("rev-parse", "HEAD"), head); assert.equal(git("diff", "--cached", "--name-only"), "file.txt");
  } finally { await page.close(); }
});
