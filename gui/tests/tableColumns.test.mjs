import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false }, esbuild: { jsx: "automatic" } });
after(() => server.close());
const { MarkdownContent, tableColumnPercents } = await server.ssrLoadModule("/src/chat/MarkdownContent.tsx");
const render = (content) => renderToStaticMarkup(createElement(MarkdownContent, { content }));

test("column widths mix an even split with each column text share", () => {
  assert.deepEqual(tableColumnPercents([]), []);
  assert.deepEqual(tableColumnPercents([0, 0]), [50, 50]);
  assert.deepEqual(tableColumnPercents([10, 10]), [50, 50]);
  const [left, right] = tableColumnPercents([1, 9]);
  assert.equal(left, 30);
  assert.equal(right, 70);
  assert.equal(Math.round((left + right) * 100) / 100, 100);
  const three = tableColumnPercents([1, 1, 1]);
  assert.equal(Math.round(three.reduce((sum, value) => sum + value, 0) * 100) / 100, 100);
  assert.ok(three.every((value) => value > 33 && value < 34));
});

test("markdown tables size columns from the text inside them", () => {
  const html = render("| Short | Longer text here |\n| --- | --- |\n| A | BBBB |\n");
  const percents = tableColumnPercents([6, 20]);
  assert.match(html, /<colgroup>/);
  for (const percent of percents) assert.match(html, new RegExp("width:" + percent + "%"));
  assert.match(html, /table-layout|chat-table-wrap/);
  assert.ok(percents[0] > 20 && percents[0] < 50);
  assert.ok(percents[1] > percents[0]);
});
