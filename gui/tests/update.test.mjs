import assert from "node:assert/strict";
import { test, after } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

const root = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({ root, configFile: false, server: { middlewareMode: true, hmr: false }, esbuild: { jsx: "automatic" } });
const { UpdateButton } = await server.ssrLoadModule("/src/shell/UpdateButton.tsx");
after(() => server.close());
const render = (status) => renderToStaticMarkup(createElement(UpdateButton, { update: { status, busy: status.phase === "downloading", error: "", act() {} } }));

test("renders actual download percentage without a spinner", () => {
  const html = render({ phase: "downloading", downloaded: 25, total: 100 });
  assert.match(html, /aria-valuenow="25"/);
  assert.match(html, /width:25%/);
  assert.doesNotMatch(html, /spinner/);
});

test("shows transferred bytes when the download size is unknown", () => {
  const html = render({ phase: "downloading", downloaded: 1048576, total: -1 });
  assert.match(html, /1.0 MB/);
  assert.doesNotMatch(html, /aria-valuenow/);
});

test("hides progress after download completion", () => {
  assert.doesNotMatch(render({ phase: "ready", downloaded: 100, total: 100 }), /progressbar/);
});
