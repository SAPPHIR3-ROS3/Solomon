import assert from "node:assert/strict";
import { after, test } from "node:test";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { readWorkspaceFileReference } = await server.ssrLoadModule("/src/chat/fileReferences.ts");
const { gitStatusFolderStatuses, mergeGitFileStatuses, fileBrowserStatuses } = await server.ssrLoadModule("/src/shell/right-panel/status.ts");

test("workspace drops accept references only for the active project", () => {
  const payload = JSON.stringify({ projectID: "project", path: "src/file with spaces.ts" });
  assert.deepEqual(readWorkspaceFileReference(payload, "project"), { projectID: "project", path: "src/file with spaces.ts" });
  assert.equal(readWorkspaceFileReference(payload, "other"), null);
  assert.equal(readWorkspaceFileReference(payload), null);
  for (const invalid of ["invalid", "null", '{}', '{"projectID":"project","path":42}']) {
    assert.equal(readWorkspaceFileReference(invalid, "project"), null);
  }
});

test("Git folders aggregate mixed changes as modified", () => {
  assert.deepEqual(gitStatusFolderStatuses({ "src/new.ts": "U", "src/nested/changed.ts": "M", "src/nested/deleted.ts": "D" }), { src: "M", "src/nested": "M" });
});

test("Git folders preserve uniform changes and normalize untracked additions", () => {
  assert.deepEqual(gitStatusFolderStatuses({ "src/new.ts": "U", "src/nested/added.ts": "A" }), { src: "A", "src/nested": "A" });
  assert.deepEqual(gitStatusFolderStatuses({ "src/nested/deleted.ts": "D" }), { src: "D", "src/nested": "D" });
});

test("Git folder aggregation is independent of file order", () => {
  const changes = { "src/new.ts": "A", "src/deleted.ts": "D", "src/changed.ts": "M" };
  assert.deepEqual(gitStatusFolderStatuses(changes), { src: "M" });
  assert.deepEqual(gitStatusFolderStatuses(Object.fromEntries(Object.entries(changes).reverse())), { src: "M" });
});

test("Modified files override additions and deletions across all ancestors", () => {
  assert.deepEqual(gitStatusFolderStatuses({ "src/new.ts": "U", "src/nested/file.ts": "M" }), { src: "M", "src/nested": "M" });
  assert.deepEqual(gitStatusFolderStatuses({ "src/deleted.ts": "D", "src/file.ts": "M" }), { src: "M" });
});

test("Staged additions cannot hide unstaged modifications", () => {
  const files = mergeGitFileStatuses({ "src/file.ts": "M" }, { "src/file.ts": "A" });
  assert.deepEqual(files, { "src/file.ts": "M" });
  assert.deepEqual(gitStatusFolderStatuses(files), { src: "M" });
});


test("File browser ignores deletions while Git retains them", () => {
  const changes = { "product/old.txt": "D", "product/new/file.txt": "U" };
  const staged = { "deleted/file.txt": "D" };
  const files = fileBrowserStatuses(changes, staged);
  assert.deepEqual(files, { "product/new/file.txt": "U" });
  assert.deepEqual(gitStatusFolderStatuses(files), { product: "A", "product/new": "A" });
  assert.deepEqual(gitStatusFolderStatuses(staged), { deleted: "D" });
  assert.deepEqual(changes, { "product/old.txt": "D", "product/new/file.txt": "U" });
});

test("File browser keeps modified folders yellow and deleted-only folders undecorated", () => {
  assert.deepEqual(fileBrowserStatuses({ "src/old.ts": "D" }), {});
  const files = fileBrowserStatuses({ "src/old.ts": "D", "src/new.ts": "U", "src/edited.ts": "M" });
  assert.deepEqual(gitStatusFolderStatuses(files), { src: "M" });
});
