import { serverEndpoint } from "../platform";

export async function terminalSocketUrl(
  workingDirectory = "",
  sessionID = "",
  after = 0,
  cols = 0,
  rows = 0,
) {
  const query = new URLSearchParams();
  if (workingDirectory) query.set("path", workingDirectory);
  if (sessionID) query.set("session_id", sessionID);
  if (after > 0) query.set("after", String(after));
  if (cols > 0) query.set("cols", String(cols));
  if (rows > 0) query.set("rows", String(rows));

  const endpoint = await serverEndpoint(`/__solomon/terminal${query.size ? `?${query}` : ""}`);
  const url = new URL(endpoint, window.location.href);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  return url.toString();
}
