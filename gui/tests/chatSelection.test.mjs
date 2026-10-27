import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

const source = readFileSync(new URL("../src/App.tsx", import.meta.url), "utf8");
const start = source.indexOf("const openProjectChat = useCallback(async (project: Project, chatID: string) => {");
const end = source.indexOf("}, [chatRuntime.openProjectChat", start);
const body = source.slice(start, end);

for (const page of ["ActiveAgents", "Customization"]) {
  test(`selecting the current chat exits ${page} before the cached-chat return`, () => {
    const reset = body.indexOf(`setIs${page}Open(false)`);
    assert.ok(reset >= 0);
    assert.ok(reset < body.indexOf("if (!isDifferentChat)"));
  });
}

test("selecting the current chat restores the agent view without starting a new load", () => {
  assert.ok(body.indexOf('setActiveView("agent")') < body.indexOf("if (!isDifferentChat)"));
  assert.ok(body.indexOf("beginNavigation(project.id, chatID)") > body.indexOf("return;"));
});
