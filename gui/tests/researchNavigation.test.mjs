import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { canOpenResearch } = await server.ssrLoadModule("/src/research/navigation.ts");
const { projectResearchFromPayload } = await server.ssrLoadModule("/src/projects/projectPayloads.ts");

test("failed research retains the cause and blocks navigation with an error notification", () => {
  const [research] = projectResearchFromPayload([{ id: "failed", title: "Niche comparison", status: "failed", error: "web search returned no results" }]);
  const errors = [];
  assert.equal(canOpenResearch(research, error => errors.push(error)), false);
  assert.deepEqual(errors, ["Deep Research failed: web search returned no results"]);
});

test("failed research without a cause uses its title", () => {
  const errors = [];
  assert.equal(canOpenResearch({ title: "Comparison", status: "failed" }, error => errors.push(error)), false);
  assert.deepEqual(errors, ["Deep Research failed: Comparison"]);
});

test("available research still opens without a notification", () => {
  for (const status of ["running", "done"]) {
    assert.equal(canOpenResearch({ title: "Comparison", status }, () => assert.fail("unexpected notification")), true);
  }
});
