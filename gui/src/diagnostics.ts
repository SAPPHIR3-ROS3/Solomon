import { serverEndpoint } from "./platform";

type Stage = "runtime" | "promise" | "react-uncaught" | "react-caught" | "react-recoverable" | "startup-identities" | "background-refresh" | "settings-save" | "autocomplete";

let windowStarted = 0;
let sent = 0;

export function reportGUIError(error: unknown, stage: Stage, filename = "", line = 0, column = 0): void {
  if (error instanceof Error && error.name === "AbortError") return;
  if (Date.now() - windowStarted >= 60_000) {
    windowStarted = Date.now();
    sent = 0;
  }
  if (sent >= 10) return;
  sent += 1;
  const name = error instanceof Error ? error.name : "UnknownError";
 if (!filename && error instanceof Error && typeof error.stack === "string") {
 const frame = error.stack.split("\n").slice(1).map((row) => row.match(/(?:^|[/\\])([A-Za-z0-9_.-]+\.(?:[cm]?js|tsx?))(?:\?[^:\s)]*)?:(\d+):(\d+)/)).find((match) => match !== null);
 if (frame) { filename = frame[1]; line = Number(frame[2]); column = Number(frame[3]); }
 }
  const source = filename.split(/[?#]/, 1)[0].split("/").pop() ?? "";
  void (async () => {
    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), 3000);
    try {
      await fetch(await serverEndpoint("/__solomon/client-diagnostics"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ stage, name, source, line, column }),
        signal: controller.signal,
      });
    } catch {
      console.warn("Solomon GUI diagnostic delivery failed", { stage, name, source, line, column });
    } finally {
      window.clearTimeout(timeout);
    }
  })();
}

export function installDiagnostics(): void {
  window.addEventListener("error", (event) => {
    reportGUIError(event.error, "runtime", event.filename, event.lineno, event.colno);
  });
  window.addEventListener("unhandledrejection", (event) => {
    reportGUIError(event.reason, "promise");
  });
}
