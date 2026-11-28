import readline from "node:readline";
import * as interaction from './interaction.mjs';
import * as navigation from './navigation.mjs';
import * as diagnostics from './diagnostics/index.mjs';

let browser;
let context;
let usingObscura = false;
let nextTabID = 1;
const tabs = new Map();
const metadata = new Map();
const contexts = new Map();
const inflight = new Map();
const obscuraURL = process.env.SOLOMON_OBSCURA_CDP_URL;

function errorMessage(error) {
  const message = error instanceof Error ? error.message : String(error);
  const [summary, callLog] = message.split(/\r?\nCall log:\r?\n/, 2);
  if (error?.name === 'TimeoutError' && callLog?.includes('intercepts pointer events')) {
    return summary.trim() + ' Another element blocked pointer events. Take a new snapshot and check for an overlay or dialog.';
  }
  return summary.trim();
}

async function ensureContext() {
  if (context) return;
  let obscuraFailure;
  if (obscuraURL) {
    try {
      const { chromium } = await import("playwright-core");
      const token = process.env.OBSCURA_CDP_TOKEN;
      browser = await chromium.connectOverCDP(obscuraURL, {
        headers: token ? { Authorization: 'Bearer ' + token } : {},
        timeout: 5000,
      });
      context = await browser.newContext();
      usingObscura = true;
      return;
    } catch (error) {
      obscuraFailure = errorMessage(error);
      if (browser) await browser.close().catch(() => undefined);
      browser = undefined;
    }
  }
  usingObscura = false;
  try {
    const { launch } = await import("cloakbrowser");
    browser = await launch({ headless: true, humanize: false, geoip: false });
    context = await browser.newContext();
  } catch (error) {
    if (browser) await browser.close().catch(() => undefined);
    browser = undefined;
    if (obscuraFailure) throw new Error('Obscura initialization failed: ' + obscuraFailure + '; CloakBrowser fallback failed: ' + errorMessage(error));
    throw error;
  }
}

function pageFor(tabID) {
  const page = tabs.get(String(tabID ?? ""));
  if (!page) throw new Error(`unknown Cloak tab ${String(tabID ?? "")}`);
  return page;
}

async function newTab(contextName) {
  await ensureContext();
  if (tabs.size >= 32) throw new Error('Cloak tab limit reached');
  let target = context;
  if (contextName) {
    if (!contexts.has(contextName)) {
      if (contexts.size >= 8) throw new Error('Cloak context limit reached');
      contexts.set(contextName, await browser.newContext());
    }
    target = contexts.get(contextName);
  }
  const page = await target.newPage();
  const tabID = `t${nextTabID++}`;
  tabs.set(tabID, page);
  metadata.set(tabID, { status: 0, contentType: "" });
  page.on("close", () => {
    void interaction.clear(tabID);
    navigation.clear(tabID);
    void diagnostics.cleanup(tabID);
    inflight.delete(tabID);
    tabs.delete(tabID);
    metadata.delete(tabID);
  });
  try {
    await navigation.attach(page, tabID, !usingObscura);
    diagnostics.attach(page, tabID);
  } catch (error) {
    await page.close().catch(() => undefined);
    throw error;
  }
  return { tabID, url: page.url(), title: "" };
}

async function navigate(args) {
  await ensureContext();
  const tabID = String(args?.tabID ?? "");
  const page = pageFor(tabID);
  const timeoutMs = Number.isFinite(args?.timeoutMs) && args.timeoutMs > 0
    ? args.timeoutMs
    : 30000;
  const response = await page.goto(navigation.safeURL(String(args?.url ?? "")), {
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
    retryAfter: headers["retry-after"] ?? "",
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
  if (page) {
    await diagnostics.cleanup(tabID);
    await page.close();
  }
  tabs.delete(tabID);
  metadata.delete(tabID);
  return { tabID, closed: true };
}

async function shutdown() {
  for (const tabID of [...tabs.keys()]) await closeTab({ tabID }).catch(() => undefined);
  for (const isolated of contexts.values()) await isolated.close().catch(() => undefined);
  contexts.clear();
  tabs.clear();
  metadata.clear();
  if (context) await context.close().catch(() => undefined);
  context = undefined;
  if (browser) await browser.close().catch(() => undefined);
  browser = undefined;
}

async function dispatch(operation, args) {
  switch (operation) {
    case 'tool':
      return callTool(args.name, args.parameters || {});
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

async function callTool(name, args) {
  if ((usingObscura || (!context && obscuraURL)) && name.includes('heapsnapshot')) {
    return {
      ok: false,
      status: 'planned',
      message: 'Heap snapshot tools will be implemented for Obscura when an official release can export V8 heap snapshots.',
      upstream: 'https://github.com/h4ckf0r0day/obscura/pull/1593',
    };
  }
  if ((usingObscura || (!context && obscuraURL)) && name === 'configure_webauthn') {
    return {
      ok: false,
      status: 'planned',
      message: 'Virtual WebAuthn authenticators are planned for Obscura once an official release supports them.',
    };
  }
  if (name === 'list_pages') {
    return { pages: await Promise.all([...tabs].map(async ([pageId, page]) => ({ pageId, url: page.url(), title: await page.title().catch(() => '') }))) };
  }
  if (name === 'new_page') {
    const result = await newTab(args.isolatedContext);
    try {
      const navigationResult = args.url ? await callTool('navigate_page', { pageId: result.tabID, url: args.url, timeout: args.timeout }) : {};
      return { ...navigationResult, pageId: result.tabID, url: pageFor(result.tabID).url(), background: true };
    } catch (error) {
      await closeTab({ tabID: result.tabID });
      throw error;
    }
  }
  const tabID = String(args.pageId ?? args.tabID ?? '');
  const page = pageFor(tabID);
  if (name === 'close_page') return closeTab({ tabID });
  if (name === 'select_page') return { pageId: tabID, url: page.url(), background: true };
  const pendingDialog = navigation.pending(tabID);
  if (pendingDialog && name !== 'handle_dialog') return { pageId: tabID, blockedByDialog: pendingDialog };
  let listener;
  const dialogEvent = new Promise(resolve => { listener = dialog => resolve({ pageId: tabID, pendingDialog: { type: dialog.type(), message: dialog.message().slice(0, 4000) } }); page.once('dialog', listener); });
  const execute = async () => {
    const state = { page, context: page.context(), browser, tabID, obscura: usingObscura, resolve: uid => interaction.resolve(page, tabID, uid) };
    for (const module of [interaction, navigation, diagnostics]) {
      const parameters = { ...args };
      if (module === diagnostics) { delete parameters.pageId; delete parameters.tabID; }
      const result = await module.dispatch(name, parameters, state);
      if (result !== undefined) return result;
    }
    throw new Error('Unknown Cloak tool ' + name);
  };
  const operation = execute();
  if (name !== 'handle_dialog') inflight.set(tabID, operation);
  try {
    const result = await Promise.race([operation, dialogEvent]);
    if (name === 'handle_dialog' && inflight.has(tabID)) {
      await inflight.get(tabID).catch(() => undefined);
      inflight.delete(tabID);
    } else if (!result.pendingDialog) { inflight.delete(tabID); }
    return result;
  }
  catch (error) {
    if (inflight.get(tabID) === operation) inflight.delete(tabID);
    throw error;
  } finally { page.off('dialog', listener); }
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
