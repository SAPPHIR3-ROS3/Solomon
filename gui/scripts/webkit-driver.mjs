import { spawn } from "node:child_process";
import { createInterface } from "node:readline";
import { fileURLToPath } from "node:url";

// An isolated GTK window under Xvfb: the same WebKitGTK engine as Wails on
// Linux, without attaching to or restarting the user's desktop application.
export async function launchDesktopWebKit() {
  const child = spawn("xvfb-run", ["-a", "/usr/bin/python3", fileURLToPath(new URL("./webkit-driver.py", import.meta.url))], { stdio: ["pipe", "pipe", "pipe"], env: {...process.env, GDK_BACKEND: "x11", WAYLAND_DISPLAY: "", SOLOMON_TEST_ISOLATED_DISPLAY: "1"} });
  const pending = new Map();
  let sequence = 0;
  let stderr = "";
  child.stderr.on("data", chunk => { stderr = (stderr + chunk).slice(-4000); });
  const fail = error => { for (const task of pending.values()) { clearTimeout(task.timer); task.reject(error); } pending.clear(); };
  child.on("error", fail);
  child.on("exit", code => fail(new Error(`WebKitGTK driver exited (${code}): ${stderr}`)));
  createInterface({ input: child.stdout }).on("line", line => {
    let reply;
    try { reply = JSON.parse(line); } catch { return; }
    const task = pending.get(reply.id);
    if (!task) return;
    pending.delete(reply.id);
    clearTimeout(task.timer);
    if (reply.error) task.reject(new Error(reply.error)); else task.resolve(reply.value);
  });
  const request = (method, args = {}) => new Promise((resolve, reject) => {
    const id = ++sequence;
    const timer = setTimeout(() => { pending.delete(id); reject(new Error(`WebKitGTK ${method} timed out: ${stderr}`)); }, 60000);
    pending.set(id, { resolve, reject, timer });
    child.stdin.write(JSON.stringify({ id, method, ...args }) + "\n");
  });
  const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
  let timeout = 60000;
  const evaluate = (fn, arg) => request("evaluate", { script: `JSON.parse(JSON.stringify(((${fn.toString()})(${JSON.stringify(arg) ?? "undefined"})) ?? null))` });
  const waitForFunction = async (fn, arg) => {
    const deadline = Date.now() + timeout;
    while (Date.now() < deadline) { if (await evaluate(fn, arg)) return; await pause(20); }
    throw new Error(`WebKitGTK condition timed out: ${fn.toString()}`);
  };
  const version = await request("version");
  return {
    version: () => version,
    newPage: async () => ({
      setDefaultTimeout: value => { timeout = value; },
      on: () => {}, // Errors are collected in the injected page script below.
      goto: async url => {
        await request("goto", { url });
        const deadline = Date.now() + timeout;
        while (!await request("loaded")) { if (Date.now() > deadline) throw new Error("WebKitGTK load timed out"); await pause(20); }
      },
      evaluate, waitForFunction, waitForTimeout: pause,
      nativeDrop: (paths, x, y) => request("nativeDrop", {paths, x, y}),
    }),
    close: async () => {
      if (child.exitCode === null) { await request("close"); child.stdin.end(); }
    },
  };
}
