import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { stripTypeScriptTypes } from "node:module";
import { test } from "node:test";

const source = await readFile(new URL("../bundleBudgets.ts", import.meta.url), "utf8");
const { bundleBudgetsPlugin, largeVendorGroups } = await import(`data:text/javascript;base64,${Buffer.from(stripTypeScriptTypes(source)).toString("base64")}`);

function warningsFor(size, modules) {
  const warnings = [];
  bundleBudgetsPlugin().generateBundle.call({ warn: message => warnings.push(message) }, {}, {
    test: { type: "chunk", fileName: "test.js", code: "x".repeat(size), modules: Object.fromEntries(modules.map(id => [id, {}])) },
  });
  return warnings;
}

test("bundle budgets keep the 500 kB warning for application and unrelated vendor code", () => {
  for (const module of ["/src/App.tsx", "/node_modules/other/index.js"]) {
    assert.equal(warningsFor(500_000, [module]).length, 0);
    assert.match(warningsFor(500_001, [module])[0], /500 kB bundle budget/);
  }
});

test("only isolated Mermaid dependencies receive the larger budgets", () => {
  const elk = "/node_modules/elkjs/lib/elk.bundled.js";
  const parser = "/node_modules/@mermaid-js/parser/dist/chunks/mermaid-parser.core/parser.mjs";
  assert.equal(warningsFor(1_500_000, [elk]).length, 0);
  assert.match(warningsFor(1_500_001, [elk])[0], /1500 kB bundle budget/);
  assert.equal(warningsFor(700_000, [parser]).length, 0);
  assert.match(warningsFor(700_001, [parser])[0], /700 kB bundle budget/);
  assert.match(warningsFor(600_000, [elk, "/src/App.tsx"])[0], /500 kB bundle budget/);
});


test("precompiled ZenUML modules get isolated bounded budgets across nested installs", () => {
  for (const prefix of ["/node_modules", "/node_modules/@mermaid-js/mermaid-zenuml/node_modules"]) {
    for (const [file, limit, group] of [["zenuml.esm.mjs", 1_800_000, "zenuml-runtime"], ["cloud-icons-newHash.js", 1_700_000, "zenuml-icons"]]) {
      const module = `${prefix}/@zenuml/core/dist/${file}`;
      assert.equal(warningsFor(limit, [module]).length, 0);
      assert.equal(warningsFor(limit + 1, [module]).length, 1);
      assert.match(warningsFor(500_001, [module, "/src/App.tsx"])[0], /500 kB bundle budget/);
      assert.equal(largeVendorGroups.find(entry => entry.test.test(module)).name, group);
    }
  }
});

test("Vite warning limit follows the largest declared dependency budget", () => {
  assert.equal(bundleBudgetsPlugin().config().build.chunkSizeWarningLimit, 1800);
});
