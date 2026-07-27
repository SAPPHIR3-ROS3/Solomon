import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { terminalPtyPlugin } from "./terminalPtyPlugin";
import { projectsPlugin } from "./projectsPlugin";
import { customizationPlugin } from "./customizationPlugin";

export default defineConfig({
  plugins: [react(), terminalPtyPlugin(), projectsPlugin(), customizationPlugin()],
});
