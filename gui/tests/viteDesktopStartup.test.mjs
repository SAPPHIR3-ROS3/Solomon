import assert from "node:assert/strict";
import { test } from "node:test";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

test("desktop Vite starts with the tracked icon when Go is absent from PATH", async () => {
  const directory = await mkdtemp(join(tmpdir(), "solomon-vite-desktop-"));
  try {
    const script = `import {createServer} from 'vite';
      const server=await createServer({cacheDir:${JSON.stringify(directory)},server:{middlewareMode:true,hmr:false}});
      if(!server.config.plugins.some(plugin=>plugin.name==='solomon-root-icon')) throw new Error('Root icon plugin missing');
      await server.close();console.log('desktop-vite-ready');`;
    const { stdout } = await promisify(execFile)(process.execPath, ["--input-type=module", "-e", script], {
      cwd: fileURLToPath(new URL("..", import.meta.url)), env: { ...process.env, PATH: directory }, timeout: 30000,
    });
    assert.match(stdout, /desktop-vite-ready/);
  } finally { await rm(directory, { recursive: true, force: true }); }
});
