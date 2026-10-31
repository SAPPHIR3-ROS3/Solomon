import assert from "node:assert/strict";
import { test, after } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false }, esbuild: { jsx: "automatic" } });
after(() => server.close());
const { ReasoningControl } = await server.ssrLoadModule("/src/chat/ReasoningControl.tsx");
const { reasoningFromPayload, resolveReasoning } = await server.ssrLoadModule("/src/projects/reasoning.ts");
function render(capability, extra = {}) {
 return renderToStaticMarkup(createElement(ReasoningControl, { capability, value: "low", open: true, onChange() {}, ...extra }));
}
test("unknown levels expose classic fallback and explicit warning", () => {
 const html = render(undefined);
 assert.match(html, /reasoning levels unknown/);
 assert.match(html, /welcome-reasoning-unknown/);
 assert.match(html, /max="5"/);
 assert.doesNotMatch(html, /Provider default/);
});
test("known subset starts at zero and popup packs slider and Fast", () => {
 const cap = { known: true, levels: ["low", "high"] };
 const html = render(cap, { fastAvailable: true });
 assert.doesNotMatch(html, /reasoning levels unknown/);
 assert.match(html, /--reasoning-scale-width:44px/);
 assert.match(html, /min="0"/);
 assert.match(html, /max="1"/);
 assert.match(html, /value="0"/);
 assert.match(html, />Fast</);
 assert.doesNotMatch(html, /<small>Medium<|<small>Max</);
});
test("single level never divides by zero", () => {
 const html = render({ known: true, levels: ["high"] }, { value: "high" });
 assert.match(html, /disabled=""/);
 assert.doesNotMatch(html, /NaN|Infinity/);
});
test("known unsupported hides slider but retains Fast", () => {
 const html = render({ known: true, levels: [] }, { fastAvailable: true });
 assert.doesNotMatch(html, /type="range"|reasoning levels unknown/);
 assert.match(html, /Not supported/);
 assert.match(html, />Fast</);
});
test("payload preserves minimal, normalizes order and resolves invalid selection", () => {
 const cap = reasoningFromPayload({ model: { known: true, levels: ["high", "minimal", "high"], default: "high" } }).model;
 assert.deepEqual(cap.levels, ["minimal", "high"]);
 assert.equal(resolveReasoning("max", cap), "high");
 assert.equal(resolveReasoning("low", { known: true, levels: [] }), undefined);
});

test("Fast indicator requires model support even with a saved enabled preference", () => {
 const unsupported = render(undefined, { fastOn: true, fastAvailable: false });
 assert.doesNotMatch(unsupported, /welcome-reasoning-fast-mark is-on/);
 assert.doesNotMatch(unsupported, /fast mode on/);
 const supported = render(undefined, { fastOn: true, fastAvailable: true });
 assert.match(supported, /welcome-reasoning-fast-mark is-on/);
 assert.match(supported, /fast mode on/);
 const disabled = render(undefined, { fastOn: false, fastAvailable: true });
 assert.doesNotMatch(disabled, /welcome-reasoning-fast-mark is-on/);
});
