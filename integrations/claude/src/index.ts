import { ClaudeRunner } from "./runner.ts";
import { createSidecar } from "./server.ts";

const token = process.env.SOLOMON_CLAUDE_TOKEN || "";
if (!token) throw new Error("missing sidecar token");
const port = Number(process.env.SOLOMON_CLAUDE_PORT || "0");
const server = createSidecar(token, new ClaudeRunner(process.env.SOLOMON_CLAUDE_EXECUTABLE || ""));
server.listen(port, "127.0.0.1", () => {
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("sidecar address unavailable");
  console.log(JSON.stringify({ port: address.port, baseURL: "http://127.0.0.1:" + address.port + "/v1" }));
});
