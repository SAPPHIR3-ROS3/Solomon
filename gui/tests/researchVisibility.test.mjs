import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { activeAgentNodesFromPayload, activeAgentChatIDsFromProjects } = await server.ssrLoadModule("/src/shell/ActiveAgentsPage.tsx");

test("running research survives parsing and marks its parent chat active", () => {
  const agents = activeAgentNodesFromPayload({ kind: "research", id: "research-1", title: "Research", projectID: "project", rootChatID: "chat", status: "running", phase: "reading" });
  assert.equal(agents.length, 1);
  assert.equal(agents[0].kind, "research");
  assert.equal(agents[0].phase, "reading");
  assert.deepEqual([...activeAgentChatIDsFromProjects([{ id: "project", name: "Project", agents }])], ["chat"]);
  assert.deepEqual(activeAgentNodesFromPayload({ ...agents[0], status: "done" }), []);
});
