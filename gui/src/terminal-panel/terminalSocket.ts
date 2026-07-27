export function terminalSocketUrl() {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  const host = window.location.hostname === "wails.localhost"
    ? `127.0.0.1${window.location.port ? `:${window.location.port}` : ""}`
    : window.location.host;

  return `${protocol}//${host}/__solomon/terminal`;
}
