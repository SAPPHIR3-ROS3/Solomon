import assert from "node:assert/strict";
import test from "node:test";
import { createSidecar } from "../src/server.ts";

test("model discovery retains SDK effort levels and requests forward effort", async () => {
 let effort: string | undefined;
 const server = createSidecar("secret", {
 async models() { return ["claude-test"]; },
 async modelCatalog() { return [{ id: "claude-test", supported_reasoning_levels: ["low", "high"] }]; },
 async complete(input) { effort = input.reasoningEffort; return { content: "ok", toolCalls: [], finishReason: "stop", usage: { promptTokens: 1, completionTokens: 1 } }; },
 });
 await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
 try {
 const address = server.address();
 assert.ok(address && typeof address === "object");
 const base = `http://127.0.0.1:${address.port}`;
 const headers = { authorization: "Bearer secret", "content-type": "application/json" };
 const models = await (await fetch(base + "/models", { headers })).json();
 assert.deepEqual(models.data[0].supported_reasoning_levels, ["low", "high"]);
 const response = await fetch(base + "/v1/chat/completions", { method: "POST", headers, body: JSON.stringify({ model: "claude-test", reasoning_effort: "high", messages: [{ role: "user", content: "hello" }] }) });
 assert.equal(response.status, 200);
 assert.equal(effort, "high");
 } finally { await new Promise<void>((resolve) => server.close(() => resolve())); }
});
