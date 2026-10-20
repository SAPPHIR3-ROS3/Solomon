import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { renderToStaticMarkup } from "react-dom/server";
import { createElement } from "react";

const root = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({ root, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { InputModeIcons } = await server.ssrLoadModule("/src/home/ModelControl.tsx");

test("missing input metadata displays no icons", () => {
  const html = renderToStaticMarkup(createElement(InputModeIcons, { modes: [] }));
  assert.equal(html, "");
});

test("declared modalities are rendered without duplicates", () => {
  const html = renderToStaticMarkup(createElement(InputModeIcons, { modes: ["text", "image", "image", "audio"] }));
  assert.match(html, /aria-label="Text input, Image input, Audio input"/);
  assert.equal((html.match(/aria-label="Image input"/g) ?? []).length, 1);
});
