import { createServer } from "./server.js";

const apiKey = process.env.CURSOR_API_KEY?.trim();
if (!apiKey) {
  console.error("CURSOR_API_KEY is required");
  process.exit(1);
}

const port = parseInt(process.env.CURSOR_API_PORT ?? "8766", 10);
const cwd = process.env.CURSOR_API_CWD?.trim() || process.cwd();

process.on("uncaughtException", (err) => {
  console.error(err);
});
process.on("unhandledRejection", (err) => {
  console.error(err);
});

const server = createServer({ apiKey, cwd });
server.listen(port, "127.0.0.1");
