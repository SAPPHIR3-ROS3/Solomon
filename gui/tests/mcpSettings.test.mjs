import assert from "node:assert/strict";
import { test, after } from "node:test";
import { mkdtemp, readFile, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const root = fileURLToPath(new URL("..", import.meta.url));
const loader = await createServer({ root, configFile: false, server: { middlewareMode: true, hmr: false } });
const { customizationPlugin } = await loader.ssrLoadModule("/customizationPlugin.ts");
const { sameCustomizationCatalog } = await loader.ssrLoadModule("/src/customization/rules.ts");
const directory = await mkdtemp(path.join(tmpdir(), "solomon-mcp-settings-"));
const configPath = path.join(directory, "mcp.json");
const previousPath = process.env.SOLOMON_MCP_CONFIG;
process.env.SOLOMON_MCP_CONFIG = configPath;
const server = await createServer({ root, configFile: false, plugins: [customizationPlugin()], server: { host: "127.0.0.1", port: 0, hmr: false } });
await server.listen();
const endpoint = `http://127.0.0.1:${server.httpServer.address().port}/__solomon/mcps`;
after(async () => {
  await server.close();
  await loader.close();
  if (previousPath === undefined) delete process.env.SOLOMON_MCP_CONFIG;
  else process.env.SOLOMON_MCP_CONFIG = previousPath;
  await rm(directory, { recursive: true, force: true });
});

test("development GUI persists MCP toggles while preserving configuration", async () => {
  const original = { extension: { keep: true }, mcpServers: { one: { command: "test", env: { TOKEN: "$SECRET" }, custom: 42 }, two: { command: "test" } } };
  await writeFile(configPath, JSON.stringify(original));
  for (const disabled of [true, false]) {
    const response = await fetch(endpoint, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ id: "one", disabled }) });
    assert.equal(response.status, 200);
    const payload = await response.json();
    assert.equal(payload.mcps.find((item) => item.id === "one").disabled, disabled);
    assert.equal(JSON.stringify(payload).includes("$SECRET"), false);
    const expected = structuredClone(original);
    expected.mcpServers.one.disabled = disabled;
    assert.deepEqual(JSON.parse(await readFile(configPath, "utf8")), expected);
    const refreshed = await (await fetch(endpoint)).json();
    assert.equal(refreshed.mcps.find((item) => item.id === "one").disabled, disabled);
  }
  const before = await readFile(configPath, "utf8");
  for (const body of [{ id: "one" }, { id: "missing", disabled: true }, { id: "one", disabled: "true" }]) {
    const response = await fetch(endpoint, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    assert.equal(response.status, 400);
  }
  assert.equal(await readFile(configPath, "utf8"), before);
});

test("catalog polling detects externally changed MCP status", () => {
  const item = { id: "one", title: "one", detail: "test" };
  assert.equal(sameCustomizationCatalog([item], [{ ...item, disabled: false }]), true);
  assert.equal(sameCustomizationCatalog([item], [{ ...item, disabled: true }]), false);
});
