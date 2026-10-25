import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";

const control = await readFile(new URL("../src/home/ProviderQuotaControl.tsx", import.meta.url), "utf8");
const settings = await readFile(new URL("../src/settings/ModelsPage.tsx", import.meta.url), "utf8");

test("quota control refreshes every minute and clears its timer", () => {
  assert.match(control, /setInterval\(\(\) => setRevision\(\(value\) => value \+ 1\), 60 \* 1000\)/);
  assert.match(control, /return \(\) => window.clearInterval\(timer\)/);
});

test("hover triggers a refresh without a manual refresh button", () => {
  assert.match(control, /onMouseEnter=\{\(\) => setRevision\(\(value\) => value \+ 1\)\}/);
  assert.match(control, /\[open, provider, revision\]/);
  assert.doesNotMatch(control, />Refresh<|: "Refresh"/);
});

test("model settings retain their two minute refresh interval", () => {
  assert.match(settings, /2 \* 60 \* 1000/);
});
