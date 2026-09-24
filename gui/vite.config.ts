import { defineConfig } from "vite";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import type { Plugin } from "vite";
import { projectsPlugin } from "./projectsPlugin";
import { customizationPlugin } from "./customizationPlugin";
import { modelsPlugin } from "./modelsPlugin";

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

export default defineConfig({
  plugins: [react(), projectsPlugin(), customizationPlugin(), modelsPlugin(), rootIconPlugin()],
  server: {
    fs: {
      allow: [".."],
    },
  },
});
