import assert from "node:assert/strict";
import { after, test } from "node:test";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { readWorkspaceFileReference } = await server.ssrLoadModule("/src/chat/fileReferences.ts");
const { gitStatusFolderStatuses } = await server.ssrLoadModule("/src/shell/right-panel/status.ts");

test("workspace drops accept references only for the active project", () => {
  const payload = JSON.stringify({ projectID: "project", path: "src/file with spaces.ts" });
  assert.deepEqual(readWorkspaceFileReference(payload, "project"), { projectID: "project", path: "src/file with spaces.ts" });
  assert.equal(readWorkspaceFileReference(payload, "other"), null);
  assert.equal(readWorkspaceFileReference(payload), null);
  for (const invalid of ["invalid", "null", '{}', '{"projectID":"project","path":42}']) {
    assert.equal(readWorkspaceFileReference(invalid, "project"), null);
  }
});

test("Git indicators propagate to every ancestor with consistent priority", () => {
  assert.deepEqual(gitStatusFolderStatuses({ "src/new.ts": "U", "src/nested/changed.ts": "M", "src/nested/deleted.ts": "D" }), { src: "D", "src/nested": "D" });
});
