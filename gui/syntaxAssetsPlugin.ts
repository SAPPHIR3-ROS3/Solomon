import { readFile } from "node:fs/promises";
import { basename } from "node:path";
import { pathToFileURL } from "node:url";
import type { Plugin } from "vite";

// Large grammar literals cannot be split between JavaScript chunks. Keep their
// data in local assets, fetched only when the existing dynamic import is used.
const LARGE_GRAMMAR_BYTES = 250_000;

export function syntaxAssetsPlugin(): Plugin {
  return {
    name: "solomon-syntax-assets",
    apply: "build",
    enforce: "pre",
    async load(id) {
      const normalized = id.replaceAll("\\", "/");
      const isCursorGrammar = /\/src\/syntax\/grammars\/[^/]+\.json\?raw$/.test(normalized);
      const isBundledGrammar = /\/node_modules\/@shikijs\/langs\/dist\/[^/]+\.mjs$/.test(normalized);
      const isWasm = normalized.endsWith("/node_modules/@shikijs/engine-oniguruma/dist/wasm-inlined.mjs");
      if (!isCursorGrammar && !isBundledGrammar && !isWasm) return;

      const path = id.split("?", 1)[0];
      if (isWasm) {
        const { wasmBinary } = await import(pathToFileURL(path).href);
        const asset = this.emitFile({ type: "asset", name: "oniguruma.wasm", source: wasmBinary });
        return `
          export async function getWasmInstance(imports) {
            const response = await fetch(import.meta.ROLLUP_FILE_URL_${asset});
            if (!response.ok) throw new Error('Unable to load syntax engine: ' + response.status);
            const { instance } = await WebAssembly.instantiate(await response.arrayBuffer(), imports);
            return instance.exports;
          }
          export default getWasmInstance;
        `;
      }

      const contents = await readFile(path);
      if (contents.byteLength < LARGE_GRAMMAR_BYTES) return;
      const source = isCursorGrammar ? contents : JSON.stringify((await import(pathToFileURL(path).href)).default);
      const asset = this.emitFile({ type: "asset", name: `syntax-${basename(path).replace(/\.mjs$/, ".json")}`, source });
      // Preserve each module's original default export: raw JSON text for the
      // Cursor snapshot and parsed grammar definitions for the Shiki catalog.
      return `
        const response = await fetch(import.meta.ROLLUP_FILE_URL_${asset});
        if (!response.ok) throw new Error('Unable to load syntax grammar: ' + response.status);
        export default await response.${isCursorGrammar ? "text" : "json"}();
      `;
    },
  };
}
