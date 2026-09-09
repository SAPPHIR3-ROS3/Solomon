import readline from "node:readline";

let browser;
let context;
let nextTabID = 1;
const tabs = new Map();
const metadata = new Map();

function errorMessage(error) {
  if (error instanceof Error) return error.stack ?? error.message;
  return String(error);
}

async function ensureContext() {
  if (context) return;
  const { launch } = await import("cloakbrowser");
  browser = await launch({
    headless: true,
    humanize: false,
    geoip: false,
  });
  context = await browser.newContext();
}

function pageFor(tabID) {
  const page = tabs.get(String(tabID ?? ""));
  if (!page) throw new Error(`unknown Cloak tab ${String(tabID ?? "")}`);
  return page;
}

async function newTab() {
  await ensureContext();
  const page = await context.newPage();
  const tabID = `t${nextTabID++}`;
  tabs.set(tabID, page);
  metadata.set(tabID, { status: 0, contentType: "" });
  page.on("close", () => {
    tabs.delete(tabID);
    metadata.delete(tabID);
  });
  return { tabID, url: page.url(), title: "" };
}

async function navigate(args) {
  await ensureContext();
  const tabID = String(args?.tabID ?? "");
  const page = pageFor(tabID);
  const timeoutMs = Number.isFinite(args?.timeoutMs) && args.timeoutMs > 0
    ? args.timeoutMs
    : 30000;
  const response = await page.goto(String(args?.url ?? ""), {
    waitUntil: "domcontentloaded",
    timeout: timeoutMs,
  });
  // Let client-rendered search/result pages settle without sending a
  // Playwright wait command through the browser protocol.
  await new Promise((resolve) => setTimeout(resolve, 350));
  const headers = response?.headers?.() ?? {};
  metadata.set(tabID, {
    status: response?.status?.() ?? 0,
    contentType: headers["content-type"] ?? "",
  });
  return {
    tabID,
    url: page.url(),
    status: response?.status?.() ?? 0,
    contentType: headers["content-type"] ?? "",
  };
}

async function snapshot(args) {
  await ensureContext();
  const tabID = String(args?.tabID ?? "");
  const page = pageFor(tabID);
  const maxCharacters = Number.isFinite(args?.maxCharacters) && args.maxCharacters > 0
    ? Math.floor(args.maxCharacters)
    : 1000000;
  // Keep this sidecar intentionally boring. Browser-specific work belongs in
  // the official Playwright wrapper; extraction, parsing, filtering and
  // result shaping stay in Solomon's Go process.
  // Always return HTML so Go owns every DOM interpretation path, including
  // callers that only want text and links in the public Snapshot value.
  const htmlValue = await page.content().catch(() => "");
  return {
    tabID,
    url: page.url(),
    text: "",
    html: String(htmlValue ?? "").slice(0, maxCharacters),
    truncated: String(htmlValue ?? "").length > maxCharacters,
    status: metadata.get(tabID)?.status ?? 0,
    contentType: metadata.get(tabID)?.contentType ?? "",
  };
}

async function closeTab(args) {
  const tabID = String(args?.tabID ?? "");
  const page = tabs.get(tabID);
  if (page) await page.close().catch(() => undefined);
  tabs.delete(tabID);
  metadata.delete(tabID);
  return { tabID, closed: true };
}

async function shutdown() {
  for (const page of tabs.values()) await page.close().catch(() => undefined);
  tabs.clear();
  metadata.clear();
  if (context) await context.close().catch(() => undefined);
  context = undefined;
  if (browser) await browser.close().catch(() => undefined);
  browser = undefined;
}

async function dispatch(operation, args) {
  switch (operation) {
    case "newTab":
      return newTab();
    case "navigate":
      return navigate(args);
    case "snapshot":
      return snapshot(args);
    case "closeTab":
      return closeTab(args);
    case "shutdown":
      await shutdown();
      return { closed: true };
    default:
      throw new Error(`unknown Cloak operation ${String(operation)}`);
  }
}

function respond(id, result, error) {
  const response = { id };
  if (error) response.error = errorMessage(error);
  else response.result = result ?? null;
  process.stdout.write(`${JSON.stringify(response)}\n`);
}

let queue = Promise.resolve();
const input = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
input.on("line", (line) => {
  queue = queue.then(async () => {
    let request;
    try {
      request = JSON.parse(line);
      const result = await dispatch(request.operation, request.args ?? {});
      respond(request.id, result, null);
    } catch (error) {
      respond(request?.id ?? null, null, error);
    }
  });
});
input.on("close", () => {
  void queue.then(() => shutdown());
});
process.on("SIGTERM", () => {
  void shutdown().finally(() => process.exit(0));
});
process.on("SIGINT", () => {
  void shutdown().finally(() => process.exit(130));
});
