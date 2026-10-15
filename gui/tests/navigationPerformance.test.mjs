import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const metrics = await server.ssrLoadModule("/src/chat/navigationPerformance.ts");

test("summaries separate completed samples and calculate median and p95", () => {
  const samples = [10, 30, 20, 50, 40].map(paintOpportunityMs => ({ outcome: "ready", paintOpportunityMs }));
  samples.push({ outcome: "cancelled", paintOpportunityMs: 10000 }, { outcome: "pending" });
  assert.deepEqual(metrics.navigationSummary(samples), { count: 5, medianMs: 30, p95Ms: 50, maxMs: 50 });
  assert.equal(metrics.navigationSummary([]).medianMs, undefined);
});

test("history is bounded, exported snapshots are independent and superseded navigation is cancelled", () => {
  metrics.clearNavigationPerformance();
  for (let index = 0; index < 105; index++) metrics.beginNavigation("project", `chat-${index}`);
  const report = metrics.navigationReport();
  assert.equal(report.samples.length, 100);
  assert.equal(report.samples[0].chatID, "chat-5");
  assert.equal(report.samples.at(-2).outcome, "cancelled");
  assert.equal(report.samples.at(-1).outcome, "pending");
  report.samples.at(-1).chatID = "mutated";
  assert.equal(metrics.navigationReport().samples.at(-1).chatID, "chat-104");
  metrics.clearNavigationPerformance();
});

test("phase measurements preserve results and exceptions, and record failed operations", async () => {
  metrics.clearNavigationPerformance();
  const id = metrics.beginNavigation("project", "chat");
  assert.equal(await metrics.measureNavigation(id, "success", async () => 42), 42);
  const error = new Error("private failure message");
  await assert.rejects(metrics.measureNavigation(id, "failure", async () => { throw error; }), reason => reason === error);
  assert.equal(metrics.measureNavigationSync(id, "sync", () => 7), 7);
  const report = metrics.navigationReport();
  assert.deepEqual(report.samples[0].spans.map(span => span.outcome), ["ok", "error", "ok"]);
  assert.ok(report.samples[0].spans.every(span => span.durationMs >= 0));
  assert.ok(!JSON.stringify(report).includes(error.message));
  metrics.cancelNavigation(id);
  metrics.failNavigation(id);
  assert.equal(metrics.navigationReport().samples[0].outcome, "cancelled");
  assert.equal(metrics.navigationForChat("project", "chat"), undefined);
  metrics.clearNavigationPerformance();
});
