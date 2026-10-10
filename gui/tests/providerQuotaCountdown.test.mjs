import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const root = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({ root, configFile: false, server: { middlewareMode: true, hmr: false } });
const { liveQuotaDetail } = await server.ssrLoadModule("/src/projects/models.ts");
after(() => server.close());

test("Cursor reset countdown updates from its date, including cached countdowns", () => {
  const detail = "Includes Cursor Grok and Composer · reset 2026-10-12 13:04 (in 4d)";
  assert.equal(liveQuotaDetail(detail, new Date(2026, 9, 10, 13, 4).getTime()),
    "Includes Cursor Grok and Composer · reset 2026-10-12 13:04 (in 2d)");
  assert.equal(liveQuotaDetail(detail, new Date(2026, 9, 11, 14, 4).getTime()),
    "Includes Cursor Grok and Composer · reset 2026-10-12 13:04 (in 23h)");
  assert.equal(liveQuotaDetail(detail, new Date(2026, 9, 12, 13, 4).getTime()),
    "Includes Cursor Grok and Composer · reset 2026-10-12 13:04");
});
