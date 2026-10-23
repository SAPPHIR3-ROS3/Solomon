import assert from "node:assert/strict";
import test from "node:test";
import { nextAction, normalizeMessages } from "../src/history.ts";

test("a new text chat starts a live session", () => {
  const action = nextAction([], null, [{ role: "system", content: "Be brief" }, { role: "user", content: "Hello" }]);
  assert.equal(action.type, "start");
  if (action.type === "start") {
    assert.equal(action.system, "Be brief");
    assert.equal(action.prompt, "Hello");
  }
});

test("existing transcripts are replayed and unsupported content is rejected", () => {
  const action = nextAction([], null, [{ role: "system", content: "Be brief" }, { role: "user", content: "Hi" }, { role: "assistant", content: "", tool_calls: [{ id: "call-1", name: "search", arguments: "{\"q\":\"x\"}" }] }, { role: "tool", tool_call_id: "call-1", content: "found" }, { role: "user", content: "andata?" }]);
  assert.equal(action.type, "start");
  if (action.type === "start") {
    assert.equal(action.system, "Be brief");
    assert.match(action.prompt, /\[user\]\nHi/);
    assert.match(action.prompt, /<tool_call name="search">\{"q":"x"\}<\/tool_call>/);
    assert.match(action.prompt, /\[tool result call-1\]\nfound/);
    assert.match(action.prompt, /\[user\]\nandata\?$/);
  }
  assert.equal(nextAction([], null, [{ role: "user", content: [{ type: "image_url", image_url: { url: "data:image/png;base64,aa" } }] }]).type, "reject");
});

test("tool results continue only the matching live call", () => {
  const accepted = [{ role: "user", content: "Read it" }];
  const produced = { content: "", tools: [{ id: "call-1", name: "search", arguments: "{}" }] };
  const incoming = normalizeMessages([...accepted, { role: "assistant", tool_calls: [{ id: "call-1", function: { name: "search", arguments: "{}" } }] }, { role: "tool", tool_call_id: "call-1", content: "done" }]);
  const action = nextAction(accepted, produced, incoming);
  assert.equal(action.type, "tools");
  if (action.type === "tools") assert.deepEqual(action.results, [{ id: "call-1", output: "done" }]);
  const changed = incoming.map((message) => ({ ...message }));
  changed[1] = { ...changed[1], content: "different" };
  assert.equal(nextAction(accepted, produced, changed).type, "reject");
});
