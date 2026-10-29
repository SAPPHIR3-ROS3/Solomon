import { pagination, rejectPaths, text } from './shared.mjs';

const tabs = new Map();
const limit = 1000;
const bodyLimit = 65536;

function headers(value) {
  const entries = [];
  let remaining = 16384;
  for (const [key, val] of Object.entries(value ?? {}).slice(0, 100)) {
    const name = text(key, Math.min(256, remaining));
    remaining -= name.length;
    const content = /authorization|cookie|token|secret|api[-_]?key/i.test(key) ? '[REDACTED]' : text(val, Math.min(2048, remaining));
    remaining -= content.length;
    entries.push([name, content]);
    if (remaining <= 0) break;
  }
  return Object.fromEntries(entries);
}

function summary(entry) {
  const { reqid, generation, url, method, resourceType, startedAt, status, finished, durationMs, failure } = entry;
  return { reqid, generation, url, method, resourceType, startedAt, status, finished, durationMs, failure };
}

function publicRequest(entry) {
  const { request, response, ...record } = entry;
  return record;
}

export function attach(page, tabID) {
  const key = String(tabID);
  if (tabs.get(key)?.page === page) return;
  cleanup(key);
  const data = { page, requests: new Map(), requestIDs: new WeakMap(), messages: new Map(), next: 1, nextMessage: 1, generation: 0, dropped: 0, droppedMessages: 0, listeners: [] };
  tabs.set(key, data);
  const on = (event, handler) => { page.on(event, handler); data.listeners.push([event, handler]); };
  const trim = () => {
    for (const [id, entry] of data.requests) {
      if (entry.generation < data.generation - 2 || data.requests.size > limit) { data.requests.delete(id); data.dropped++; }
    }
  };
  on('request', request => {
    try {
      if (request.isNavigationRequest() && !request.redirectedFrom() && request.frame() === page.mainFrame()) data.generation++;
    } catch {}
    const id = data.next++;
    data.requestIDs.set(request, id);
    data.requests.set(id, { reqid: id, request, generation: data.generation, url: text(request.url(), 16384), method: request.method(), resourceType: request.resourceType(), requestHeaders: headers(request.headers()), startedAt: Date.now(), status: null, finished: false });
    trim();
  });
  on('response', response => {
    const entry = data.requests.get(data.requestIDs.get(response.request()));
    if (entry) Object.assign(entry, { response, status: response.status(), statusText: text(response.statusText(), 512), responseHeaders: headers(response.headers()), fromServiceWorker: response.fromServiceWorker() });
  });
  on('requestfinished', request => {
    const entry = data.requests.get(data.requestIDs.get(request));
    if (entry) Object.assign(entry, { finished: true, timing: request.timing(), durationMs: Date.now() - entry.startedAt });
  });
  on('requestfailed', request => {
    const entry = data.requests.get(data.requestIDs.get(request));
    if (entry) Object.assign(entry, { finished: true, failure: text(request.failure()?.errorText), durationMs: Date.now() - entry.startedAt });
  });
  const message = record => {
    const id = data.nextMessage++;
    data.messages.set(id, { msgid: id, generation: data.generation, timestamp: Date.now(), ...record });
    for (const [oldID, old] of data.messages) {
      if (old.generation < data.generation - 2 || data.messages.size > 500) { data.messages.delete(oldID); data.droppedMessages++; }
    }
  };
  on('console', item => message({ type: item.type(), text: text(item.text(), 16384), location: { ...item.location(), url: text(item.location()?.url, 8192) } }));
  on('pageerror', error => message({ type: 'error', text: text(error.message, 16384), stack: text(error.stack, 16384) }));
  on('close', () => cleanup(key));
}

export function cleanup(tabID) {
  const key = String(tabID);
  const data = tabs.get(key);
  if (!data) return;
  for (const [event, handler] of data.listeners) data.page.off(event, handler);
  tabs.delete(key);
}

async function details(entry) {
  const result = publicRequest(entry);
  result.requestBody = text(entry.request.postData(), bodyLimit);
  result.requestBodyTruncated = (entry.request.postData()?.length ?? 0) > bodyLimit;
  result.bodyLimitBytes = bodyLimit;
  result.headersMayBeTruncated = true;
  if (!entry.response || !entry.finished) { result.bodyUnavailable = 'Response has not completed'; return result; }
  result.requestHeaders = headers(await entry.request.allHeaders());
  result.responseHeaders = headers(await entry.response.allHeaders());
  const raw = entry.response.headers();
  const size = Number(raw['content-length']);
  const encoding = raw['content-encoding'];
  if (!Number.isFinite(size) || size < 0 || size > bodyLimit || raw['content-length'] === undefined || (encoding && encoding !== 'identity')) {
    result.bodyUnavailable = 'Only completed, uncompressed responses with Content-Length at most 65536 bytes are read';
    return result;
  }
  try {
    const body = await entry.response.body();
    result.responseBody = body.subarray(0, bodyLimit).toString('base64');
    result.responseBodyEncoding = 'base64';
    result.responseBodyTruncated = body.length > bodyLimit;
  } catch (error) { result.bodyUnavailable = text(error.message); }
  return result;
}

export async function dispatch(name, args = {}, state) {
  if (!['list_network_requests', 'get_network_request', 'list_console_messages', 'get_console_message'].includes(name)) return undefined;
  rejectPaths(args);
  attach(state.page, state.tabID);
  const data = tabs.get(String(state.tabID));
  if (name === 'get_network_request') {
    if (args.reqid === undefined) throw new Error('reqid is required; there is no selected DevTools request');
    const entry = data.requests.get(args.reqid);
    if (!entry) throw new Error('Unknown or evicted network request');
    return details(entry);
  }
  if (name === 'get_console_message') {
    const entry = data.messages.get(args.msgid);
    if (!entry) throw new Error('Unknown or evicted console message');
    return entry;
  }
  const network = name === 'list_network_requests';
  const filter = network ? args.resourceTypes : args.types;
  if (filter !== undefined && (!Array.isArray(filter) || filter.some(item => typeof item !== 'string'))) throw new Error('Expected an array of types');
  let entries = [...(network ? data.requests : data.messages).values()].filter(entry => args.includePreservedRequests || args.includePreservedMessages || entry.generation === data.generation);
  if (filter?.length) entries = entries.filter(entry => filter.includes(network ? entry.resourceType : entry.type));
  const result = pagination(entries, args);
  result.items = network ? result.items.map(summary) : result.items;
  return { ...result, dropped: network ? data.dropped : data.droppedMessages, captureStartedOnAttach: true, preservedNavigations: 3, textMayBeTruncated: true };
}
