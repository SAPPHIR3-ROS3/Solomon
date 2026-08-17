export function terminalSocketUrl(workingDirectory = "") {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  const host = window.location.hostname === "wails.localhost"
    ? `127.0.0.1${window.location.port ? `:${window.location.port}` : ""}`
    : window.location.host;

  const query = workingDirectory ? `?path=${encodeURIComponent(workingDirectory)}` : "";
  return `${protocol}//${host}/__solomon/terminal${query}`;
}
