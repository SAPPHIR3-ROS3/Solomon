import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { projectsPlugin } from "./projectsPlugin";
import { customizationPlugin } from "./customizationPlugin";
import { modelsPlugin } from "./modelsPlugin";

export default defineConfig({
  plugins: [react(), projectsPlugin(), customizationPlugin(), modelsPlugin()],
  server: {
    fs: {
      allow: [".."],
    },
  },
});
