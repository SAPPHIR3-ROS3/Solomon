import assert from "node:assert/strict";
import { after, test } from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const root = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({ root, configFile: false, server: { middlewareMode: true, hmr: false } });
const { SensitiveText } = await server.ssrLoadModule("/src/SensitiveText.tsx");
after(() => server.close());
const render = props => renderToStaticMarkup(createElement(SensitiveText, props));

test("sensitive text starts blurred and hides its value from accessibility labels", () => {
  const html = render({ value: "private@example.com", label: "account email" });
  assert.match(html, /--sensitive-text-blur:7px/);
  assert.match(html, /aria-hidden="true" class="sensitive-text-blurred"/);
  assert.match(html, /aria-label="Show account email"/);
  assert.match(html, /aria-pressed="false"/);
  assert.doesNotMatch(html, /(?:aria-label|title)="[^"]*private@example.com/);
});

test("non-sensitive values stay plain without a reveal control", () => {
  assert.equal(render({ value: "Team network", sensitive: false }), "Team network");
});

test("other text supports a custom blur and styling without interpreting markup", () => {
  const html = render({ value: "<identifier>", label: "identifier", blur: 5, className: "custom" });
  assert.match(html, /class="sensitive-text custom"/);
  assert.match(html, /--sensitive-text-blur:5px/);
  assert.match(html, /&lt;identifier&gt;/);
  assert.match(html, /aria-label="Show identifier"/);
});
