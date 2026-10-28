import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { isSlashCommand } = await server.ssrLoadModule("/src/chat/slashCommands.ts");

test("absolute image paths are messages, with or without spaces", () => {
  for (const value of ["/home/oni/Pictures/Screenshots/Screenshot 2026-10-28.png", " /tmp/image.webp explain this", "/home/oni/photo.jpeg", "/", "//server/share/image.png", "Read /tmp/image.png", ""]) {
    assert.equal(isSlashCommand(value), false, value);
  }
});

test("commands and skills retain their routing, including path arguments", () => {
  for (const value of ["/help", "  /agent  ", "/docs /home/oni/file.png", "/skill:review inspect /tmp/file", "/custom.skill arguments", "/unknown-command"]) {
    assert.equal(isSlashCommand(value), true, value);
  }
});
