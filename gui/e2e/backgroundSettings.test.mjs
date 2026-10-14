import assert from "node:assert/strict";
import { test } from "node:test";
import { existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { chromium } from "playwright-core";

// Test only the settings component with simulated native bindings. Never start
// or stop a real daemon, or acquire a real power inhibitor.
test("offline desktop can retry startup, persist Keep awake, and stop explicitly", async () => {
  const executablePath = process.env.SOLOMON_TEST_BROWSER || ["/usr/bin/chromium", "/usr/bin/chromium-browser"].find(existsSync);
  assert.ok(executablePath, "Chromium is required");
  const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, plugins: [{
    name: "background-settings-test-entry",
    configureServer(server) {
      server.middlewares.use("/background-test.html", (_request, response) => {
        response.setHeader("Content-Type", "text/html");
        response.end('<div id="root"></div><script type="module" src="/background-test-entry.js"></script>');
      });
    },
    resolveId(id) { if (id === "/background-test-entry.js") return "\0background-test-entry.js"; },
    load(id) {
      if (id === "\0background-test-entry.js") return `
        import React from 'react';
        import {createRoot} from 'react-dom/client';
        import {BackgroundSettings} from '/src/settings/BackgroundSettings.tsx';
        createRoot(document.getElementById('root')).render(React.createElement(BackgroundSettings));
      `;
    },
  }], server: { host: "127.0.0.1", port: 0 } });
  let browser;
  try {
    await server.listen();
    browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
    const page = await browser.newPage();
    page.setDefaultTimeout(10000);
    page.on("pageerror", error => console.error(error.message));
    page.on("console", message => { if (message.type() === "error") console.error(message.text()); });
    await page.addInitScript(() => {
      window.go = { main: { ServerBridge: {
        URL: async () => window.location.origin,
        DaemonRunning: async () => sessionStorage.getItem("running") === "true",
        SetDaemonEnabled: async (enabled) => {
          if (!sessionStorage.getItem("attempted")) {
            sessionStorage.setItem("attempted", "true");
            throw new Error("Simulated startup failure");
          }
          sessionStorage.setItem("running", String(enabled));
          return enabled;
        },
      } } };
    });
    let awake = false;
    await page.route("**/__solomon/background", async route => {
      if (route.request().method() === "PATCH") awake = route.request().postDataJSON().keepAwake;
      await route.fulfill({ json: { keepAwake: awake } });
    });
    await page.goto(`${server.resolvedUrls.local[0]}background-test.html`);
    const daemon = page.getByRole("switch", { name: "Enable daemon", exact: true });
    const keepAwake = page.getByRole("switch", { name: "Keep awake", exact: true });
    await page.waitForFunction(() => document.querySelector('[aria-labelledby="daemon-label"]')?.disabled === false);
    assert.equal(await keepAwake.isDisabled(), true);
    await daemon.click();
    await page.getByRole("alert").filter({ hasText: "Simulated startup failure" }).waitFor();
    assert.equal(await daemon.getAttribute("aria-checked"), "false");
    await Promise.all([page.waitForEvent("load"), daemon.click()]);
    await page.waitForFunction(() => document.querySelector('[aria-labelledby="awake-label"]')?.disabled === false);
    assert.equal(await daemon.getAttribute("aria-checked"), "true");
    await keepAwake.click();
    await page.waitForFunction(() => document.querySelector('[aria-labelledby="awake-label"]')?.getAttribute("aria-checked") === "true");
    assert.equal(awake, true);
    await page.reload();
    await page.waitForFunction(() => document.querySelector('[aria-labelledby="awake-label"]')?.getAttribute("aria-checked") === "true");
    page.once("dialog", dialog => dialog.accept());
    await daemon.click();
    await page.waitForFunction(() => document.querySelector('[aria-labelledby="daemon-label"]')?.getAttribute("aria-checked") === "false");
    assert.equal(await keepAwake.isDisabled(), true);
  } finally {
    await browser?.close();
    await server.close();
  }
}, { timeout: 30000 });
