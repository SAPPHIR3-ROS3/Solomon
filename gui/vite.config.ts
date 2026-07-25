import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { terminalPtyPlugin } from "./terminalPtyPlugin";
import { threadFoldersPlugin } from "./threadFoldersPlugin";

export default defineConfig({
  plugins: [react(), terminalPtyPlugin(), threadFoldersPlugin()],
});
