import assert from "node:assert/strict";
import { after, test } from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { AdvancedSettings } = await server.ssrLoadModule("/src/settings/AdvancedSettings.tsx");

test("advanced settings expose the searchable subagent recursion limit", () => {
  const html = renderToStaticMarkup(createElement(AdvancedSettings, { query: "recursion" }));
  assert.match(html, /Subagent recursion limit/);
  assert.match(html, /advanced-subagent_max_depth/);
  assert.match(html, /main agent is level 0/);
  assert.match(html, /Default: 4/);
  assert.match(html, /min="1"/);
  assert.match(html, /max="32"/);
});
