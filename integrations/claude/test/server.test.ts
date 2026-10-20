import assert from "node:assert/strict";
import { once } from "node:events";
import test from "node:test";
import { createSidecar, type Runner } from "../src/server.ts";

function runner(): Runner {
  const seen: string[][] = [];
  return {
    async models() { return ["claude-sonnet-5"]; },
    async complete(input, onDelta) {
      seen.push(input.messages.map((message) => message.role));
      if (input.messages.at(-1)?.content === "tool") {
        return { content: "", toolCalls: [{ id: "call-1", name: "search", arguments: "{}" }], finishReason: "tool_calls", usage: { promptTokens: 1, completionTokens: 1 } };
      }
      onDelta("ok");
      return { content: "ok", toolCalls: [], finishReason: "stop", usage: { promptTokens: 2, completionTokens: 1 } };
    },
  };
}

async function request(port: number, token: string, path: string, body?: unknown) {
  const response = await fetch("http://127.0.0.1:" + port + path, { method: body ? "POST" : "GET", headers: { authorization: "Bearer " + token, "content-type": "application/json" }, body: body ? JSON.stringify(body) : undefined });
  return { status: response.status, text: await response.text() };
}

test("sidecar authenticates health, models and streaming completions", async () => {
  const server = createSidecar("secret", runner());
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  try {
    const address = server.address();
    if (!address || typeof address === "string") throw new Error("no port");
    assert.equal((await request(address.port, "wrong", "/health")).status, 401);
    assert.equal((await request(address.port, "secret", "/health")).text, JSON.stringify({ ok: true }));
    assert.match((await request(address.port, "secret", "/v1/models")).text, /claude-sonnet-5/);
    const streamed = await request(address.port, "secret", "/v1/chat/completions", { model: "claude-sonnet-5", stream: true, messages: [{ role: "user", content: "Hi" }] });
    assert.equal(streamed.status, 200);
    assert.match(streamed.text, /ok/);
    assert.match(streamed.text, /\[DONE\]/);
  } finally {
    server.close();
    await once(server, "close");
  }
});
