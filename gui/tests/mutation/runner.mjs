import { spawnSync } from "node:child_process";
import { readFile, writeFile, unlink } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { createServer as viteCreateServer } from "vite";

const root = new URL("../../", import.meta.url);

export function createServer(options) {
  const mutation = process.env.SOLOMON_MUTATION ? JSON.parse(process.env.SOLOMON_MUTATION) : null;
  if (!mutation) return viteCreateServer(options);
  const target = fileURLToPath(new URL(mutation.file, root));
  return viteCreateServer({
    ...options,
    plugins: [...(options.plugins ?? []), {
      name: "controlled-mutation",
      enforce: "pre",
      transform(code, id) {
        if (id.split("?")[0] !== target) return;
        if (code.split(mutation.before).length !== 2) throw new Error("Mutation target must occur exactly once");
        return { code: code.replace(mutation.before, mutation.after), map: null };
      },
    }],
  });
}

function execute(path, mutation) {
  const env = { ...process.env };
  delete env.SOLOMON_MUTATION;
  if (mutation) env.SOLOMON_MUTATION = JSON.stringify(mutation);
  const result = spawnSync(process.execPath, ["--test", "--test-reporter=tap", path], {
    cwd: fileURLToPath(root), env, encoding: "utf8", timeout: 30000, maxBuffer: 4 * 1024 * 1024,
  });
  const output = `${result.stdout ?? ""}${result.stderr ?? ""}`;
  if (result.error || result.signal) return { status: "error", output: `${result.error ?? result.signal}\n${output}` };
  if (result.status === 0) return { status: "survived", output };
  return { status: output.includes("code: 'ERR_ASSERTION'") ? "killed" : "error", output };
}

async function main() {
  const catalog = JSON.parse(await readFile(new URL("unread.json", import.meta.url), "utf8"));
  const source = await readFile(new URL(catalog.file, root), "utf8");
  for (const mutation of catalog.mutations) {
    if (source.split(mutation.before).length !== 2 || mutation.before === mutation.after) {
      throw new Error(`Invalid mutation: ${mutation.name}`);
    }
  }
  const original = await readFile(new URL(catalog.test, root), "utf8");
  const importPattern = /import \{ createServer \} from ["']vite["'];/;
  if (!importPattern.test(original)) throw new Error("Test must import createServer from vite");
  const temporary = new URL(`tests/.mutation-${process.pid}.mjs`, root);
  const path = fileURLToPath(temporary);
  await writeFile(temporary, original.replace(importPattern, `import { createServer } from ${JSON.stringify(import.meta.url)};`), { flag: "wx" });
  try {
    const baseline = execute(path, null);
    if (baseline.status !== "survived") throw new Error(`Baseline failed:\n${baseline.output}`);
    console.log("Baseline passed");
    const counts = { killed: 0, survived: 0, error: 0 };
    for (const mutation of catalog.mutations) {
      const result = execute(path, { ...mutation, file: catalog.file });
      counts[result.status]++;
      console.log(`${result.status.toUpperCase()}: ${mutation.name}`);
      if (result.status === "error") console.error(result.output);
    }
    console.log(`Mutation score: ${counts.killed}/${catalog.mutations.length}; survivors: ${counts.survived}; errors: ${counts.error}`);
    if (counts.survived || counts.error) process.exitCode = 1;
  } finally {
    await unlink(temporary);
  }
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  await main();
}
