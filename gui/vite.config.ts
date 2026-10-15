import { defineConfig } from "vite";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import type { Plugin } from "vite";
import { projectsPlugin } from "./projectsPlugin";
import { customizationPlugin } from "./customizationPlugin";
import { guiSettingsPlugin } from "./guiSettingsPlugin";
import { modelsPlugin } from "./modelsPlugin";
import { syntaxAssetsPlugin } from "./syntaxAssetsPlugin";

const rootIconPath = fileURLToPath(new URL("../icon.svg", import.meta.url));
const repositoryRoot = fileURLToPath(new URL("..", import.meta.url));

function rootIconPlugin(): Plugin {
  const iconContents = () => readFileSync(rootIconPath);
  return {
    name: "solomon-root-icon",
    config() {
      // The desktop server can invoke Vite directly, so generate the icon here
      // before either the dev server or production build reads it.
      execFileSync("go", ["run", "scripts/generate_icon.go"], {
        cwd: repositoryRoot,
        stdio: "inherit",
      });
    },
    configureServer(server) {
      server.middlewares.use("/icon.svg", (_request, response) => {
        response.setHeader("Content-Type", "image/svg+xml; charset=utf-8");
        response.setHeader("Cache-Control", "no-cache");
        response.end(iconContents());
      });
    },
    generateBundle() {
      this.emitFile({ type: "asset", fileName: "icon.svg", source: iconContents() });
    },
  };
}

// Browser integration tests use a private dependency cache alongside their daemon.
export default defineConfig({
  define: { __SOLOMON_GUI_BUILD_ID__: JSON.stringify(new Date().toISOString()) },
  cacheDir: process.env.SOLOMON_VITE_CACHE_DIR || undefined,
  plugins: [react(), guiSettingsPlugin(), projectsPlugin(), customizationPlugin(), modelsPlugin(), rootIconPlugin(), syntaxAssetsPlugin()],
  optimizeDeps: {
    // Avoid scanning/prebundling every lazy syntax grammar on startup.
    // Prebundle the shared UI dependencies (including nested CommonJS), while
    // leaving the lazy syntax grammars as ESM loaded only when requested.
    noDiscovery: true,
    include: [
      "react", "react-dom/client", "react/jsx-runtime", "react/jsx-dev-runtime", "@xterm/xterm",
      "@codemirror/autocomplete", "@codemirror/commands", "@codemirror/language",
      "@codemirror/state", "@codemirror/view", "@xterm/addon-fit",
      "@shikijs/core", "@shikijs/engine-oniguruma", "@shikijs/engine-oniguruma/wasm-inlined",
      "react-markdown", "remark-gfm", "remark-math", "rehype-katex", "katex",
    ],
  },
  build: {
    rolldownOptions: {
      output: {
        strictExecutionOrder: true,
        codeSplitting: {
          groups: [
            // Let Vite preserve stylesheet import order (responsive overrides
            // must follow the base styles) independently of JavaScript groups.
            { name: "vendor", test: id => /node_modules/.test(id) && !/\.css(?:\?|$)/.test(id), minSize: 50_000, maxSize: 250_000, entriesAware: true, includeDependenciesRecursively: false },
            { name: "app", test: id => /[\\/]src[\\/]/.test(id) && !/\.css(?:\?|$)/.test(id), minSize: 50_000, maxSize: 250_000, entriesAware: true, includeDependenciesRecursively: false },
          ],
        },
      },
    },
  },
  server: {
    fs: {
      allow: [".."],
    },
  },
});
