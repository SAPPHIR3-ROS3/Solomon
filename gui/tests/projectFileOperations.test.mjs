import assert from "node:assert/strict";
import { after, test } from "node:test";
import { mkdtemp, mkdir, readFile, writeFile, rm, symlink } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const root = fileURLToPath(new URL("..", import.meta.url));
const fixture = await mkdtemp(path.join(tmpdir(), "solomon-file-operations-"));
const workspace = path.join(fixture, "workspace");
const home = path.join(fixture, "home");
await mkdir(workspace);
await mkdir(home);
const projectID = "a".repeat(64);
await writeFile(path.join(home, "projectsId.json"), JSON.stringify({ [workspace]: projectID }));
const previousHome = process.env.SOLOMON_HOME;
process.env.SOLOMON_HOME = home;
const loader = await createServer({ root, configFile: false, server: { middlewareMode: true, hmr: false } });
const { projectsPlugin } = await loader.ssrLoadModule("/projectsPlugin.ts");
const server = await createServer({ root, configFile: false, plugins: [projectsPlugin()], server: { host: "127.0.0.1", port: 0, hmr: false } });
await server.listen();
const endpoint = `http://127.0.0.1:${server.httpServer.address().port}/__solomon/projects/${projectID}/file-operation`;
after(async () => {
  await server.close();
  await loader.close();
  if (previousHome === undefined) delete process.env.SOLOMON_HOME;
  else process.env.SOLOMON_HOME = previousHome;
  await rm(fixture, { recursive: true, force: true });
});
const operate = (operation) => fetch(endpoint, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(operation) });

test("development API copies, renames, moves and deletes files without overwriting", async () => {
  await writeFile(path.join(workspace, "original.txt"), "content");
  for (const operation of [
    { action: "copy", path: "original.txt", destination: "copy.txt" },
    { action: "rename", path: "copy.txt", destination: "renamed.txt" },
    { action: "move", path: "renamed.txt", destination: "moved.txt" },
  ]) {
    const response = await operate(operation);
    assert.equal(response.status, 200, await response.text());
  }
  assert.equal(await readFile(path.join(workspace, "moved.txt"), "utf8"), "content");
  assert.equal((await operate({ action: "copy", path: "original.txt", destination: "moved.txt" })).status, 400);
  assert.equal((await operate({ action: "delete", path: "moved.txt" })).status, 200);
  await assert.rejects(readFile(path.join(workspace, "moved.txt")), { code: "ENOENT" });
  assert.equal(await readFile(path.join(workspace, "original.txt"), "utf8"), "content");
});

test("development API rejects workspace deletion and escaping destinations", async () => {
  for (const operation of [
    { action: "delete", path: "." },
    { action: "rename", path: "original.txt", destination: "../outside.txt" },
    { action: "copy", path: "original.txt", destination: path.join(fixture, "outside.txt") },
  ]) assert.equal((await operate(operation)).status, 400);
  await mkdir(path.join(fixture, "outside"));
  await writeFile(path.join(fixture, "outside", "file.txt"), "untouched");
  await symlink(path.join(fixture, "outside"), path.join(workspace, "escape"), "dir");
  assert.equal((await operate({ action: "delete", path: "escape/file.txt" })).status, 400);
  assert.equal((await operate({ action: "copy", path: "original.txt", destination: "escape/copy.txt" })).status, 400);
  assert.equal(await readFile(path.join(fixture, "outside", "file.txt"), "utf8"), "untouched");
});
