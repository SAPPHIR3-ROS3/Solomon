import assert from "node:assert/strict";
import { after, test } from "node:test";
import { existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { chromium } from "playwright-core";

const fixture = `import React from 'react';
import { createRoot } from 'react-dom/client';
import { useAppUpdate } from '/src/shell/useAppUpdate.ts';
import { UpdateButton } from '/src/shell/UpdateButton.tsx';
function Fixture() {
  const update = useAppUpdate();
  const [open, setOpen] = React.useState(false);
  return <><button onClick={() => setOpen(value => !value)}>Toggle panel</button>
    <output>{update.status?.phase}</output>
    {open ? <UpdateButton update={update} /> : null}</>;
}
createRoot(document.getElementById('root')).render(<Fixture />);`;
const server = await createServer({
  root: fileURLToPath(new URL("..", import.meta.url)), configFile: false,
  plugins: [{ name: "update-fixture", resolveId: id => id === "/__update.tsx" ? id : null, load: id => id === "/__update.tsx" ? fixture : null }],
  server: { host: "127.0.0.1", port: 0, hmr: false }, esbuild: { jsx: "automatic" },
});
await server.listen();
after(() => server.close());
const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/google-chrome"].find(existsSync);
const browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
after(() => browser.close());

test("checks with the panel closed and preserves the update through panel remounts", async () => {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", error => errors.push(error.message));
  try {
    await page.clock.install();
    let phase = "idle";
    let checks = 0;
    await page.route("**/__update", route => route.fulfill({ contentType: "text/html", body: '<div id="root"></div><script type="module" src="/__update.tsx"></script>' }));
    await page.route("**/__solomon/update", route => {
      if (route.request().method() === "POST") {
        phase = "ready";
      } else {
        checks++;
      }
      return route.fulfill({ json: { current: "v1", latest: "v2", phase } });
    });
    await page.goto(`${server.resolvedUrls.local[0]}__update`);
    await page.waitForFunction(() => document.querySelector("output")?.textContent === "idle");
    assert.equal(checks, 1);
    phase = "available";
    await page.clock.fastForward(30_000);
    await page.waitForFunction(() => document.querySelector("output")?.textContent === "available");
    const beforeOpening = checks;
    await page.getByRole("button", { name: "Toggle panel" }).click();
    assert.equal(await page.getByRole("button", { name: "Download update", exact: true }).count(), 1);
    assert.equal(checks, beforeOpening);
    await page.getByRole("button", { name: "Download update", exact: true }).click();
    await page.getByRole("button", { name: "Restart to update", exact: true }).waitFor();
    await page.getByRole("button", { name: "Toggle panel" }).click();
    const beforeReopening = checks;
    await page.getByRole("button", { name: "Toggle panel" }).click();
    assert.equal(await page.getByRole("button", { name: "Restart to update", exact: true }).count(), 1);
    assert.equal(checks, beforeReopening);
    assert.deepEqual(errors, []);
  } finally {
    await page.close();
  }
});
